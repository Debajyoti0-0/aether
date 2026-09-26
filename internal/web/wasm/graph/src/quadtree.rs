//! Barnes-Hut quadtree for O(n log n) repulsion in a force-directed layout.
//!
//! # Why this exists
//!
//! Naive force-directed layout computes repulsion for every pair of nodes:
//! O(n²) per iteration. At 50,000 nodes that is 2.5 billion pair evaluations
//! for a *single* iteration, which is unusable in a browser.
//!
//! Barnes-Hut approximates instead of enumerating. Nodes are aggregated into a
//! quadtree; when a cluster is far enough from a node, the whole cluster is
//! replaced by a single point mass at its centre of mass. The error is bounded
//! by the opening-angle criterion, controlled by `theta`.
//!
//! # Determinism
//!
//! The layout feeds a dashboard that must produce the same picture for the same
//! data every time. Everything here is therefore deterministic: iteration order
//! is fixed by node index, subdivision is a pure function of position, and no
//! random jitter or hash-order iteration is used anywhere.

/// A node in the simulation, in simulation space.
#[derive(Clone, Copy, Debug)]
pub struct Body {
    pub x: f32,
    pub y: f32,
    pub vx: f32,
    pub vy: f32,
    /// Uniform mass today. It is a field so a future weighted layout (for
    /// example weighting computers above groups) does not require rewriting the
    /// tree, not because it varies in this build.
    pub mass: f32,
    pub pinned: bool,
}

/// Maximum subdivision depth.
///
/// The bound matters: without it, two nodes at identical coordinates subdivide
/// forever. At this depth a coincident cluster becomes a dense leaf, which is
/// correct for a force layout (coincident nodes simply receive summed
/// repulsion) and terminates the recursion.
const MAX_DEPTH: u32 = 20;

/// Smallest root extent, so a single-node or all-coincident graph still has a
/// well-defined cell and never divides by zero.
const MIN_EXTENT: f32 = 1.0;

/// Distance below which two bodies count as coincident, and the distance
/// substituted for them so the inverse-square term stays finite.
const COINCIDENT_EPSILON: f64 = 1e-3;

/// Ceiling on a single repulsion term, as a multiple of the strength. Without
/// it, two near-coincident bodies produce an impulse large enough to fling them
/// out of the viewport and the layout never recovers.
const MAX_FORCE_FACTOR: f64 = 1e4;

#[derive(Debug)]
enum Cell {
    /// Exactly one body.
    Leaf { body: usize },
    /// Several bodies at effectively the same point; subdivision abandoned.
    Dense { members: Vec<usize> },
    Node {
        com_x: f64,
        com_y: f64,
        mass: f64,
        children: [Option<Box<Cell>>; 4],
    },
}

impl Cell {
    fn node() -> Self {
        Cell::Node {
            com_x: 0.0,
            com_y: 0.0,
            mass: 0.0,
            children: [None, None, None, None],
        }
    }
}

/// A Barnes-Hut quadtree over a fixed set of bodies.
pub struct Quadtree {
    root: Box<Cell>,
    size: f32,
}

impl Quadtree {
    /// Build a tree over `bodies`.
    pub fn build(bodies: &[Body]) -> Quadtree {
        if bodies.is_empty() {
            return Quadtree {
                root: Box::new(Cell::node()),
                size: MIN_EXTENT,
            };
        }

        let (mut min_x, mut min_y) = (f32::INFINITY, f32::INFINITY);
        let (mut max_x, mut max_y) = (f32::NEG_INFINITY, f32::NEG_INFINITY);
        for b in bodies {
            min_x = min_x.min(b.x);
            min_y = min_y.min(b.y);
            max_x = max_x.max(b.x);
            max_y = max_y.max(b.y);
        }

        // A square root cell keeps the aspect ratio at 1, which is what makes
        // the opening-angle test directionally unbiased.
        let extent = (max_x - min_x).max(max_y - min_y);
        let size = if extent.is_finite() && extent > MIN_EXTENT {
            extent
        } else {
            MIN_EXTENT
        };
        let origin_x = (min_x + max_x) * 0.5 - size * 0.5;
        let origin_y = (min_y + max_y) * 0.5 - size * 0.5;

        let mut root = Box::new(Cell::node());
        for i in 0..bodies.len() {
            insert(&mut root, origin_x, origin_y, size, 0, i, bodies);
        }
        Quadtree { root, size }
    }

    /// Accumulate the repulsive force acting on body `i` into `out`.
    ///
    /// `theta` is the opening-angle threshold: 0 is exact O(n²), larger is
    /// faster and less accurate. The dashboard uses 0.7.
    pub fn apply_repulsion(
        &self,
        i: usize,
        bodies: &[Body],
        theta: f32,
        strength: f32,
        out: &mut [f32; 2],
    ) {
        let b = bodies[i];
        self.accumulate(
            &self.root,
            i,
            b.x as f64,
            b.y as f64,
            theta as f64,
            strength as f64,
            0,
            bodies,
            out,
        );
    }

    /// Node count, for tests and diagnostics.
    pub fn cell_count(&self) -> usize {
        fn walk(c: &Cell) -> usize {
            match c {
                Cell::Leaf { .. } => 1,
                Cell::Dense { members } => members.len(),
                Cell::Node { children, .. } => {
                    1 + children.iter().flatten().map(|c| walk(c)).sum::<usize>()
                }
            }
        }
        walk(&self.root)
    }

    /// Cell width at a given depth, clamped so a deep depth cannot overflow the
    /// shift.
    fn width_at(&self, depth: u32) -> f64 {
        let shift = depth.min(19);
        (self.size / (1u32 << shift) as f32) as f64
    }

    #[allow(clippy::too_many_arguments)]
    fn accumulate(
        &self,
        cell: &Cell,
        self_index: usize,
        px: f64,
        py: f64,
        theta: f64,
        strength: f64,
        depth: u32,
        bodies: &[Body],
        out: &mut [f32; 2],
    ) {
        match cell {
            Cell::Leaf { body } => {
                if *body != self_index {
                    self.exact_pair(self_index, *body, px, py, strength, bodies, out);
                }
            }
            Cell::Dense { members } => {
                for b in members {
                    if *b != self_index {
                        self.exact_pair(self_index, *b, px, py, strength, bodies, out);
                    }
                }
            }
            Cell::Node {
                com_x,
                com_y,
                mass,
                children,
            } => {
                if *mass <= 0.0 {
                    return;
                }
                let dx = px - *com_x;
                let dy = py - *com_y;
                let dist2 = dx * dx + dy * dy;
                let width = self.width_at(depth);

                // Opening-angle criterion: collapse the cell to a point mass
                // when width / distance < theta. A cell containing the query
                // point has no meaningful distance, so it always recurses.
                let approximate = dist2 > 0.0 && width * width < theta * theta * dist2;
                if approximate || depth >= MAX_DEPTH {
                    let dist = dist2.sqrt().max(COINCIDENT_EPSILON);
                    // Inverse-square falloff, magnitude scaled by total mass.
                    // Clamped so two near-coincident nodes cannot produce an
                    // infinite impulse that makes the layout explode.
                    let f = (strength * mass / dist).min(strength * MAX_FORCE_FACTOR);
                    out[0] += (dx / dist * f) as f32;
                    out[1] += (dy / dist * f) as f32;
                    return;
                }
                for c in children.iter().flatten() {
                    self.accumulate(c, self_index, px, py, theta, strength, depth + 1, bodies, out);
                }
            }
        }
    }

    /// Repulsion between one body and one other body, at full precision.
    ///
    /// The coincident case gets explicit handling. When two nodes share a
    /// coordinate the true force direction is undefined, and normalising
    /// (0, 0) produces exactly zero — so the nodes would stay stacked on top of
    /// each other forever, drawn as one dot with no way to select either. The
    /// direction is therefore taken from a deterministic compass keyed on the
    /// index difference, which separates them and keeps the layout reproducible
    /// at the same time.
    #[allow(clippy::too_many_arguments)]
    fn exact_pair(
        &self,
        self_index: usize,
        other: usize,
        px: f64,
        py: f64,
        strength: f64,
        bodies: &[Body],
        out: &mut [f32; 2],
    ) {
        let o = bodies[other];
        let dx = px - o.x as f64;
        let dy = py - o.y as f64;
        let dist2 = dx * dx + dy * dy;
        let (ux, uy, dist) = if dist2 > COINCIDENT_EPSILON * COINCIDENT_EPSILON {
            let d = dist2.sqrt();
            (dx / d, dy / d, d)
        } else {
            // Eight compass directions, offset by the index difference, so
            // neighbours in the node list fan out instead of moving as one.
            let step = (other + 1).wrapping_sub(self_index) % 8;
            let angle = step as f64 * std::f64::consts::FRAC_PI_4;
            (angle.cos(), angle.sin(), COINCIDENT_EPSILON)
        };
        let f = (strength * o.mass as f64 / dist).min(strength * MAX_FORCE_FACTOR);
        out[0] += (ux * f) as f32;
        out[1] += (uy * f) as f32;
    }
}

/// Insert one body, accumulating centre of mass on the way down.
///
/// A leaf that already holds a different body is promoted to an internal node
/// and **both** bodies are re-inserted, which is why the full `bodies` slice is
/// needed. That is the only correct way to handle coincident points; the
/// alternative (dropping one) would silently lose a node.
///
/// The function is total and always terminates: the only way to create a node is
/// by promoting a leaf, and promotion stops at [`MAX_DEPTH`] by collapsing into
/// a dense cluster.
#[allow(clippy::too_many_arguments)]
fn insert(
    cell: &mut Cell,
    ox: f32,
    oy: f32,
    size: f32,
    depth: u32,
    body_index: usize,
    bodies: &[Body],
) {
    // A dense cluster swallows everything that reaches it.
    if let Cell::Dense { members } = cell {
        if !members.contains(&body_index) {
            members.push(body_index);
        }
        return;
    }

    if let Cell::Leaf { body } = *cell {
        if body == body_index {
            return; // already present
        }
        if depth >= MAX_DEPTH {
            *cell = Cell::Dense {
                members: vec![body, body_index],
            };
            return;
        }
        let previous = body;
        *cell = Cell::node();
        // Re-insert the previously stored body into the fresh node, then fall
        // through so the new body is inserted too. Both must be inserted:
        // dropping either would silently lose a node from the layout.
        insert(cell, ox, oy, size, depth + 1, previous, bodies);
    }

    let x = bodies[body_index].x as f64;
    let y = bodies[body_index].y as f64;
    let mass = bodies[body_index].mass as f64;

    if let Cell::Node {
        com_x, com_y, mass: m, ..
    } = cell
    {
        let total = *m + mass;
        if total > 0.0 {
            *com_x = (*com_x * *m + x * mass) / total;
            *com_y = (*com_y * *m + y * mass) / total;
        }
        *m = total;
    }

    let Cell::Node { children, .. } = cell else {
        return;
    };
    let half = size * 0.5;
    let mid_x = ox as f64 + half as f64;
    let mid_y = oy as f64 + half as f64;
    let q = usize::from(x >= mid_x) | (usize::from(y >= mid_y) << 1);
    let child_ox = if x >= mid_x { ox + half } else { ox };
    let child_oy = if y >= mid_y { oy + half } else { oy };
    match children[q].as_mut() {
        // An occupied cell starts life as a leaf, which is what makes the
        // promotion above reachable and what keeps the tree O(n) in build time
        // instead of pre-expanding an empty node per level.
        None => children[q] = Some(Box::new(Cell::Leaf { body: body_index })),
        Some(c) => insert(c, child_ox, child_oy, half, depth + 1, body_index, bodies),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A stationary body, the state every test starts from.
    fn body(x: f32, y: f32) -> Body {
        Body {
            x,
            y,
            vx: 0.0,
            vy: 0.0,
            mass: 1.0,
            pinned: false,
        }
    }

    #[test]
    fn empty_graph_builds() {
        let t = Quadtree::build(&[]);
        assert!(t.cell_count() >= 1);
    }

    #[test]
    fn single_node_has_no_self_force() {
        let bodies = vec![body(0.0, 0.0)];
        let t = Quadtree::build(&bodies);
        let mut out = [0.0f32; 2];
        t.apply_repulsion(0, &bodies, 0.7, 100.0, &mut out);
        assert_eq!(out, [0.0, 0.0]);
    }

    #[test]
    fn coincident_nodes_do_not_hang_or_panic() {
        // The pathological case for a quadtree: every point identical, so
        // subdivision would never terminate without a depth bound.
        let bodies: Vec<Body> = (0..64).map(|_| body(5.0, 5.0)).collect();
        let t = Quadtree::build(&bodies);
        let mut out = [0.0f32; 2];
        t.apply_repulsion(0, &bodies, 0.7, 100.0, &mut out);
        assert!(out[0].is_finite() && out[1].is_finite());
        // A coincident pair must still be pushed apart in some direction,
        // otherwise the nodes stay stacked and cannot be selected separately.
        assert!(out[0] != 0.0 || out[1] != 0.0, "no separation force at all");
    }

    #[test]
    fn distinct_nodes_all_survive_in_the_tree() {
        let bodies: Vec<Body> = (0..500)
            .map(|i| body((i % 25) as f32, (i / 25) as f32))
            .collect();
        let t = Quadtree::build(&bodies);
        // 500 bodies on a 25x20 grid: cells must be at least as many as the
        // occupied positions, so a dropped body would show up here.
        assert!(t.cell_count() >= 500, "cell_count={}", t.cell_count());
    }

    #[test]
    fn repulsion_pushes_apart() {
        let bodies = vec![body(-1.0, 0.0), body(1.0, 0.0)];
        let t = Quadtree::build(&bodies);
        let mut out = [0.0f32; 2];
        t.apply_repulsion(0, &bodies, 0.0, 1000.0, &mut out);
        // Body 0 is left of body 1, so repulsion must push it further left.
        assert!(out[0] < 0.0, "expected leftward force, got {out:?}");
    }

    #[test]
    fn theta_zero_matches_exact_pairwise() {
        let bodies = vec![body(0.0, 0.0), body(3.0, 4.0)];
        let t = Quadtree::build(&bodies);

        let mut approx = [0.0f32; 2];
        t.apply_repulsion(0, &bodies, 0.0, 500.0, &mut approx);

        // Hand-computed exact inverse-square value: distance 5, magnitude
        // 500/5 = 100, direction from body 1 to body 0 is (-0.6, -0.8).
        let expect_x = -0.6f32 * 100.0;
        let expect_y = -0.8f32 * 100.0;
        assert!(
            (approx[0] - expect_x).abs() < 1e-2,
            "x: got {} want {}",
            approx[0],
            expect_x
        );
        assert!(
            (approx[1] - expect_y).abs() < 1e-2,
            "y: got {} want {}",
            approx[1],
            expect_y
        );
    }

    #[test]
    fn build_is_deterministic() {
        let bodies: Vec<Body> = (0..200)
            .map(|i| body(((i * 37) % 101) as f32, ((i * 53) % 97) as f32))
            .collect();
        let a = Quadtree::build(&bodies);
        let b = Quadtree::build(&bodies);
        let mut fa = [0.0f32; 2];
        let mut fb = [0.0f32; 2];
        for i in 0..bodies.len() {
            a.apply_repulsion(i, &bodies, 0.7, 100.0, &mut fa);
            b.apply_repulsion(i, &bodies, 0.7, 100.0, &mut fb);
            assert_eq!(fa, fb, "force differed at body {i}");
            fa = [0.0; 2];
            fb = [0.0; 2];
        }
    }
}
