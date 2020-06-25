FROM bitnami/kubectl
USER 0
COPY replace.sh /replace.sh
RUN chmod +x /replace.sh
ENTRYPOINT ["/replace.sh"]
