//! Force-directed layout for the identity graph, compiled to WebAssembly.
//!
//! # Why WebAssembly
//!
//! The layout is the only part of the dashboard that is O(n²)-ish work on every
//! interaction, and it runs while the operator is dragging a graph around. Doing
//! it in JavaScript means the numbers the operator sees come from code that is
//! awkward to audit alongside the audit verifier; doing it here means the hot
//! path is a few hundred lines of arithmetic with no dependencies at all.
//!
//! # ABI
//!
//! A deliberately small C ABI over the module's linear memory. The browser side
//! (`internal/web/static/graph.js`) allocates a buffer, writes flat typed-array
//! views into it, calls [`aether_layout`], and reads the results back. No
//! generated glue, no string marshalling, no host allocation.
//!
//! Input arrays, all indexed by node index:
//!
//! | pointer    | element type | layout                    |
//! |------------|--------------|---------------------------|
//! | `node_in`  | `f32`        | 3 per node: x, y, pinned  |
//! | `edge_in`  | `u32`        | 2 per edge: source, target |
//!
//! Output arrays:
//!
//! | pointer    | element type | layout                     |
//! |------------|--------------|----------------------------|
//! | `node_out` | `f32`        | 2 per node: x, y           |
//! | `diag_out` | `u32`        | 4: cells, iterations, nodes moved, clamped |
//!
//! # Determinism
//!
//! Initial positions come from a phyllotaxis spiral indexed by node index, not
//! from a random source, so the same graph always lays out the same way. An
//! operator who screenshots a graph twice should get the same picture, and
//! evidence artifacts should be reproducible.

mod quadtree;

use quadtree::{Body, Quadtree};

/// Return codes from [`aether_layout`].
const OK: i32 = 0;
const ERR_NULL: i32 = -1;
const ERR_COUNT: i32 = -2;
const ERR_ZERO: i32 = -3;

/// Largest node count the layout will accept.
///
/// The bound is a memory-safety guard, not a product limit: the caller's buffers
/// are sized by the caller, and a mismatch between the declared count and the
/// actual buffer is undefined behaviour in any ABI that takes raw pointers. A
/// ceiling means a corrupt count produces a clean error instead of a trap.
const MAX_NODES: u32 = 500_000;

/// Allocate `bytes` bytes in the module's linear memory.
///
/// The size is rounded up to a 16-byte boundary so the pointer satisfies
/// WebAssembly's alignment expectations for every type the caller writes.
#[no_mangle]
pub extern "C" fn aether_alloc(bytes: u32) -> *mut u8 {
    let n = (bytes as usize + 15) & !15;
    let mut buf: Vec<u8> = Vec::with_capacity(n);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Release a buffer previously returned by [`aether_alloc`].
///
/// The length must be the one passed to `aether_alloc`; the layout is rebuilt
/// identically from the rounded capacity, so the same rounding is applied here.
#[no_mangle]
pub extern "C" fn aether_dealloc(ptr: *mut u8, bytes: u32) {
    if ptr.is_null() {
        return;
    }
    let n = (bytes as usize + 15) & !15;
    // SAFETY: the caller promises this pointer came from aether_alloc with this
    // length, which is the contract of an exported deallocator.
    unsafe {
        drop(Vec::from_raw_parts(ptr, 0, n));
    }
}

/// Run the layout.
///
/// Returns [`OK`] or a negative error code. `diag_out` may be null when the
/// caller does not want the diagnostics.
#[no_mangle]
#[allow(clippy::too_many_arguments)]
pub extern "C" fn aether_layout(
    node_count: u32,
    node_in: *const f32,
    edge_count: u32,
    edge_in: *const u32,
    iterations: u32,
    repulsion: f32,
    attraction: f32,
    damping: f32,
    theta: f32,
    max_speed: f32,
    node_out: *mut f32,
    diag_out: *mut u32,
) -> i32 {
    if node_in.is_null() || node_out.is_null() {
        return ERR_NULL;
    }
    if node_count > MAX_NODES {
        return ERR_COUNT;
    }
    if edge_count > 0 && edge_in.is_null() {
        return ERR_NULL;
    }
    if node_count == 0 {
        return ERR_ZERO;
    }
    // SAFETY: the caller guarantees node_in holds 3*node_count f32 values and
    // node_out has room for 2*node_count. Both counts are bounds-checked above
    // and the raw-pointer ABI is the caller's contract.
    let (bodies, moved, clamped, cells) = unsafe {
        layout_impl(
            node_count,
            std::slice::from_raw_parts(node_in, node_count as usize * 3),
            edge_count,
            if edge_count == 0 {
                &[]
            } else {
                std::slice::from_raw_parts(edge_in, edge_count as usize * 2)
            },
            iterations,
            repulsion,
            attraction,
            damping,
            theta,
            max_speed,
        )
    };
    // SAFETY: the caller guarantees node_out has room for 2*node_count f32
    // values, and node_count was bounds-checked above.
    let out = unsafe { std::slice::from_raw_parts_mut(node_out, node_count as usize * 2) };
    for (i, body) in bodies.iter().enumerate() {
        out[i * 2] = body.x;
        out[i * 2 + 1] = body.y;
    }
    if !diag_out.is_null() {
        // SAFETY: the caller guarantees room for 4 u32 diagnostics.
        let d = unsafe { std::slice::from_raw_parts_mut(diag_out, 4) };
        d[0] = cells;
        d[1] = iterations;
        d[2] = moved;
        d[3] = clamped;
    }
    OK
}

/// The layout itself, in safe Rust, so it can be unit tested on the host.
///
/// Returns the laid-out bodies, the number of nodes that actually moved, the
/// number of velocity clamps applied (a non-zero clamp count means forces are
/// still saturating, which the caller can surface as "did not settle"), and the
/// cell count of the final tree.
#[allow(clippy::too_many_arguments)]
fn layout_impl(
    node_count: u32,
    node_in: &[f32],
    edge_count: u32,
    edge_in: &[u32],
    iterations: u32,
    repulsion: f32,
    attraction: f32,
    damping: f32,
    theta: f32,
    max_speed: f32,
) -> (Vec<Body>, u32, u32, u32) {
    let n = node_count as usize;
    let mut bodies: Vec<Body> = Vec::with_capacity(n);

    // A caller may hand us a degenerate seed: every node at the origin, or a
    // graph that is flat on one axis. Those coordinates are finite, so a naive
    // "is it finite" check accepts them, and then the inverse-square repulsion
    // between coincident bodies is either infinite or, after the clamps, a
    // division by a near-zero distance, and the layout returns garbage or an
    // error. The dashboard in fact seeds every node at the origin on a first
    // render, so this is the common case and not a corner case.
    //
    // The fix is to treat a zero-extent seed as no seed at all and fall back to
    // the phyllotaxis placement, which is deterministic and already spread out.
    let mut min_x = f32::INFINITY;
    let mut max_x = f32::NEG_INFINITY;
    let mut min_y = f32::INFINITY;
    let mut max_y = f32::NEG_INFINITY;
    let mut finite = 0usize;
    for i in 0..n {
        let (x, y) = (node_in[i * 3], node_in[i * 3 + 1]);
        if x.is_finite() && y.is_finite() {
            finite += 1;
            min_x = min_x.min(x);
            max_x = max_x.max(x);
            min_y = min_y.min(y);
            max_y = max_y.max(y);
        }
    }
    // A single node, or a seed with no extent, is a stack. 1e-3 is far below any
    // real layout scale and far above f32 noise around zero.
    let degenerate = finite < 2 || (max_x - min_x) < 1e-3 || (max_y - min_y) < 1e-3;

    // Initial placement: phyllotaxis spiral. Deterministic, and it starts nodes
    // spread out rather than stacked at the origin, which matters because a
    // stacked start makes inverse-square forces enormous for the first few
    // iterations.
    for i in 0..n {
        let (x, y) = if !degenerate
            && node_in[i * 3].is_finite()
            && node_in[i * 3 + 1].is_finite()
        {
            (node_in[i * 3], node_in[i * 3 + 1])
        } else {
            seed_position(i, n)
        };
        bodies.push(Body {
            x,
            y,
            vx: 0.0,
            vy: 0.0,
            mass: 1.0,
            pinned: node_in[i * 3 + 2] != 0.0,
        });
    }

    let mut edges: Vec<(usize, usize)> = Vec::with_capacity(edge_count as usize);
    for e in 0..edge_count as usize {
        let s = edge_in[e * 2] as usize;
        let t = edge_in[e * 2 + 1] as usize;
        // An out-of-range or self-referential index would panic on slice
        // indexing, and a self-loop has no force in it. Dropping the edge is
        // the safe reading: it cannot be drawn as a line either.
        if s < n && t < n && s != t {
            edges.push((s, t));
        }
    }

    let mut moved = 0u32;
    let mut clamped = 0u32;
    let mut cells = 0u32;

    for _ in 0..iterations {
        let tree = Quadtree::build(&bodies);
        cells = tree.cell_count().min(u32::MAX as usize) as u32;

        let mut fx = vec![0.0f32; n];
        let mut fy = vec![0.0f32; n];

        // Repulsion, approximated by the tree.
        for i in 0..n {
            let mut out = [0.0f32; 2];
            tree.apply_repulsion(i, &bodies, theta, repulsion, &mut out);
            fx[i] += out[0];
            fy[i] += out[1];
        }

        // Attraction along edges. A spring that shortens with distance keeps
        // connected objects together without a rest length, so no per-edge
        // tuning is needed and the layout has fewer magic numbers.
        for &(s, t) in &edges {
            let dx = bodies[t].x - bodies[s].x;
            let dy = bodies[t].y - bodies[s].y;
            let dist = (dx * dx + dy * dy).sqrt().max(1e-3);
            // Clamp the pull so a long edge across the whole graph does not
            // dominate every other force.
            let f = (attraction * dist.min(200.0)).min(attraction * 200.0);
            let ux = dx / dist * f;
            let uy = dy / dist * f;
            fx[s] += ux;
            fy[s] += uy;
            fx[t] -= ux;
            fy[t] -= uy;
        }

        // Centring. Without it, disconnected components drift apart until the
        // view is mostly empty space.
        for i in 0..n {
            fx[i] -= bodies[i].x * 0.02;
            fy[i] -= bodies[i].y * 0.02;
        }

        for i in 0..n {
            if bodies[i].pinned {
                bodies[i].vx = 0.0;
                bodies[i].vy = 0.0;
                continue;
            }
            let mut vx = (bodies[i].vx + fx[i]) * damping;
            let mut vy = (bodies[i].vy + fy[i]) * damping;
            let speed = (vx * vx + vy * vy).sqrt();
            if speed > max_speed {
                let k = max_speed / speed;
                vx *= k;
                vy *= k;
                clamped += 1;
            }
            if !vx.is_finite() || !vy.is_finite() {
                // A single NaN would otherwise propagate through the tree and
                // blank the whole picture. Reset that node and carry on.
                vx = 0.0;
                vy = 0.0;
            }
            bodies[i].vx = vx;
            bodies[i].vy = vy;
            bodies[i].x += vx;
            bodies[i].y += vy;
            if !bodies[i].x.is_finite() || !bodies[i].y.is_finite() {
                let (sx, sy) = seed_position(i, n);
                bodies[i].x = sx;
                bodies[i].y = sy;
            }
            if vx != 0.0 || vy != 0.0 {
                moved += 1;
            }
        }
    }

    (bodies, moved, clamped, cells)
}

/// Deterministic starting position for node `i` of `n`, on a phyllotaxis spiral.
fn seed_position(i: usize, n: usize) -> (f32, f32) {
    if n <= 1 {
        return (0.0, 0.0);
    }
    let t = (i as f32 + 0.5) / n as f32;
    let radius = 400.0 * t.sqrt();
    let angle = i as f32 * 2.399_963_23_f32;
    (radius * angle.cos(), radius * angle.sin())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn run(n: u32, edges: &[(u32, u32)], iters: u32) -> Vec<Body> {
        let mut input = vec![0.0f32; n as usize * 3];
        for i in 0..n as usize {
            let (x, y) = seed_position(i, n as usize);
            input[i * 3] = x;
            input[i * 3 + 1] = y;
        }
        let flat: Vec<u32> = edges.iter().flat_map(|(a, b)| [*a, *b]).collect();
        let (bodies, _, _, _) =
            layout_impl(n, &input, edges.len() as u32, &flat, iters, 300.0, 0.05, 0.85, 0.7, 12.0);
        bodies
    }

    #[test]
    fn single_node_stays_at_origin() {
        let b = run(1, &[], 50);
        assert_eq!(b.len(), 1);
        assert!(b[0].x.is_finite() && b[0].y.is_finite());
    }

    #[test]
    fn layout_produces_finite_coordinates() {
        // A star graph: one hub, many leaves. This is the shape a real identity
        // graph has around Domain Admins, and it is the case that produces the
        // largest forces.
        let edges: Vec<(u32, u32)> = (1..64).map(|i| (0, i)).collect();
        let b = run(64, &edges, 120);
        for (i, body) in b.iter().enumerate() {
            assert!(body.x.is_finite() && body.y.is_finite(), "node {i} not finite");
        }
    }

    /// The dashboard seeds every node at the origin on a first render, so an
    /// all-coincident input is the default path, not an edge case. It used to
    /// return an error from the ABI, which left the graph page stuck on
    /// "loading graph…" with no explanation.
    #[test]
    fn an_all_coincident_seed_still_lays_out() {
        let n = 5u32;
        let edges = [(0, 2), (1, 2), (4, 2), (2, 3), (4, 3)];
        let input = vec![0.0f32; n as usize * 3];
        let flat: Vec<u32> = edges.iter().flat_map(|(a, b)| [*a, *b]).collect();
        let (bodies, _, _, _) =
            layout_impl(n, &input, edges.len() as u32, &flat, 150, 300.0, 0.05, 0.85, 0.7, 12.0);
        assert_eq!(bodies.len(), n as usize);
        for (i, body) in bodies.iter().enumerate() {
            assert!(body.x.is_finite() && body.y.is_finite(), "node {i} not finite");
        }
        // Spread out, not still stacked: a layout that returns the origin five
        // times has not laid anything out. The coordinates are hashed as their
        // bit patterns because f32 is not itself hashable.
        let distinct = bodies
            .iter()
            .map(|b| (b.x.to_bits(), b.y.to_bits()))
            .collect::<std::collections::HashSet<_>>();
        assert!(distinct.len() > 1, "every node landed on the same position");
    }

    /// A graph flat on one axis has the same problem in one dimension: the
    /// spring force along the degenerate axis is zero and the repulsion has
    /// nothing to separate it with.
    #[test]
    fn a_flat_seed_still_lays_out() {
        let n = 6u32;
        let mut input = vec![0.0f32; n as usize * 3];
        for i in 0..n as usize {
            input[i * 3] = i as f32;
        }
        let edges = [(0u32, 1u32), (1, 2), (3, 4), (4, 5)];
        let flat: Vec<u32> = edges.iter().flat_map(|(a, b)| [*a, *b]).collect();
        let (bodies, _, _, _) =
            layout_impl(n, &input, edges.len() as u32, &flat, 150, 300.0, 0.05, 0.85, 0.7, 12.0);
        for (i, body) in bodies.iter().enumerate() {
            assert!(body.x.is_finite() && body.y.is_finite(), "node {i} not finite");
        }
    }

    /// A caller may pass garbage coordinates; they must not poison the layout.
    #[test]
    fn non_finite_seeds_are_replaced() {
        let n = 4u32;
        let mut input = vec![0.0f32; n as usize * 3];
        input[3] = f32::NAN;
        input[7] = f32::INFINITY;
        input[9] = f32::NEG_INFINITY;
        input[1] = -12.5;
        input[4] = 7.25;
        let edges = [(0u32, 1u32), (2, 3)];
        let flat: Vec<u32> = edges.iter().flat_map(|(a, b)| [*a, *b]).collect();
        let (bodies, _, _, _) =
            layout_impl(n, &input, edges.len() as u32, &flat, 120, 300.0, 0.05, 0.85, 0.7, 12.0);
        for (i, body) in bodies.iter().enumerate() {
            assert!(body.x.is_finite() && body.y.is_finite(), "node {i} not finite");
        }
    }

    #[test]
    fn edge_ends_stay_nearer_than_unconnected_pairs() {
        let mut edges = Vec::new();
        for i in 0..20u32 {
            edges.push((i, i + 20));
        }
        let b = run(40, &edges, 200);

        let d = |a: &Body, b: &Body| {
            let dx = a.x - b.x;
            let dy = a.y - b.y;
            (dx * dx + dy * dy).sqrt()
        };
        let mut linked = f32::MAX;
        for i in 0..20u32 {
            linked = linked.min(d(&b[i as usize], &b[(i + 20) as usize]));
        }
        // Any two unconnected nodes, as a comparison baseline.
        let mut unlinked = 0.0f32;
        for i in 0..20u32 {
            unlinked += d(&b[i as usize], &b[((i + 7) % 20 + 20) as usize]);
        }
        unlinked /= 20.0;
        assert!(
            linked < unlinked,
            "linked pairs (avg nearest {linked}) should be closer than unconnected baseline ({unlinked})"
        );
    }

    #[test]
    fn layout_is_deterministic() {
        let edges: Vec<(u32, u32)> = (1..30u32).map(|i| (i / 3, i)).collect();
        let a = run(30, &edges, 100);
        let b = run(30, &edges, 100);
        for i in 0..a.len() {
            assert_eq!(a[i].x, b[i].x, "node {i} x differs between runs");
            assert_eq!(a[i].y, b[i].y, "node {i} y differs between runs");
        }
    }

    #[test]
    fn pinned_nodes_do_not_move() {
        let mut input = vec![0.0f32; 3 * 3];
        input[0] = 0.0;
        input[1] = 0.0;
        input[2] = 1.0; // pinned
        input[3] = 50.0;
        input[4] = 0.0;
        input[5] = 1.0; // pinned
        input[6] = 0.0;
        input[7] = 50.0;
        input[8] = 0.0;
        let (bodies, _, _, _) =
            layout_impl(3, &input, 0, &[], 100, 300.0, 0.05, 0.85, 0.7, 12.0);
        assert_eq!(bodies[0].x, 0.0);
        assert_eq!(bodies[0].y, 0.0);
        assert_eq!(bodies[1].x, 50.0);
        assert_eq!(bodies[1].y, 0.0);
        assert!(bodies[2].x != 0.0 || bodies[2].y != 50.0);
    }

    #[test]
    fn out_of_range_edges_are_dropped_not_fatal() {
        // A malformed edge must not trap: the picture loses one line, the page
        // still renders.
        let b = run(4, &[(0u32, 99u32), (1, 2)], 20);
        assert_eq!(b.len(), 4);
        for body in &b {
            assert!(body.x.is_finite());
        }
    }

    #[test]
    fn coincident_nodes_separate() {
        // Every node at the same point is the pathological case for a force
        // layout; the clamp in the quadtree is what keeps it finite.
        let input = vec![0.0f32; 3 * 10];
        let (bodies, _, _, _) =
            layout_impl(10, &input, 0, &[], 60, 300.0, 0.05, 0.85, 0.7, 12.0);
        let mut distinct = 0;
        for i in 0..bodies.len() {
            assert!(bodies[i].x.is_finite() && bodies[i].y.is_finite());
            let unique = bodies
                .iter()
                .enumerate()
                .all(|(j, o)| j == i || o.x != bodies[i].x || o.y != bodies[i].y);
            if unique {
                distinct += 1;
            }
        }
        assert!(distinct > 0, "coincident nodes never separated");
    }
}
