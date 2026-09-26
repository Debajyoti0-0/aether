# Stage 45b — Custom Samba AD DC Image
# Based on Ubuntu 22.04 with Samba AD DC provisioned

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV SAMBA_DOMAIN=AETHER
ENV SAMBA_REALM=AETHER.TEST
ENV ADMIN_PASSWORD=Passw0rd123!
ENV KERBEROS_ENCRYPTION_TYPES=aes256-cts-hmac-sha1-96,aes128-cts-hmac-sha1-96,rc4-hmac

# Install Samba AD DC and dependencies
RUN apt-get update && apt-get install -y \
    samba \
    samba-dsdb-modules \
    samba-vfs-modules \
    winbind \
    libnss-winbind \
    libpam-winbind \
    krb5-user \
    krb5-config \
    ldap-utils \
    net-tools \
    iputils-ping \
    dnsutils \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /var/lib/samba /var/log/samba /etc/samba /var/run/samba

# Copy provisioning script
COPY provision.sh /provision.sh
RUN chmod +x /provision.sh

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose ports
EXPOSE 53/tcp 53/udp 88/tcp 88/udp 135/tcp 139/tcp 389/tcp 389/udp 445/tcp 464/tcp 464/udp 636/tcp 3268/tcp 3269/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=5 --start-period=60s \
    CMD samba-tool domain info AETHER.TEST || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/sbin/samba", "--foreground", "--no-process-group"]