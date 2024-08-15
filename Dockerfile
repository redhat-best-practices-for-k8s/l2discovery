FROM fedora:40
RUN dnf -y install iputils-20240117-4.fc40 iproute-6.7.0-2.fc40 procps-ng- 4.0.4-3.fc40 tcpdump-14:4.99.4-7.fc40  ethtool-2:6.10-1.fc40 pciutils-3.13.0-1.fc40;dnf clean all
COPY l2discovery /usr/bin
CMD ["/bin/sh", "-c", "/usr/bin/l2discovery"]
