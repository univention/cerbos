# SPDX-FileCopyrightText: 2026 Univention GmbH
# SPDX-License-Identifier: AGPL-3.0-only

ARG DOCKER_PROXY
FROM ${DOCKER_PROXY}alpine:3.24@sha256:71e9b7bae160ba167fa17414401b94cbcdaf3c32feeab9298bd794a2a0614c35 AS certs
RUN apk add -U --no-cache ca-certificates && update-ca-certificates

FROM scratch
EXPOSE 3592 3593
ENV CERBOS_CONFIG="__default__"
VOLUME ["/policies", "/tmp", "/.cache"]
ENTRYPOINT ["/cerbos"]
CMD ["server"]
HEALTHCHECK --interval=10s --timeout=2s --retries=2 CMD ["/cerbos", "healthcheck"]
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY cerbos /cerbos
