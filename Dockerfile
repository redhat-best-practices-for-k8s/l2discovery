FROM fedora
RUN dnf -y install iputils iproute procps tcpdump ethtool
COPY l2discovery /usr/bin
USER 0
CMD ["/bin/sh", "-c", "/usr/bin/l2discovery"]
