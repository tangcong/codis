FROM golang:1.10 as builder

RUN apt-get update
RUN apt-get install -y autoconf

ENV GOPATH /gopath
ENV CODIS  ${GOPATH}/src/github.com/CodisLabs/codis
ENV PATH   ${GOPATH}/bin:${PATH}:${CODIS}/bin
COPY . ${CODIS}

RUN make -C ${CODIS} distclean
RUN make -C ${CODIS} build-all


# Copy the binary into a thin image
FROM centos:latest
ENV GOPATH /gopath
ENV CODIS  ${GOPATH}/src/github.com/CodisLabs/codis
ENV PATH   ${GOPATH}/bin:${PATH}:${CODIS}/bin
RUN mkdir -p ${CODIS}
WORKDIR ${CODIS}
COPY --from=builder /gopath/src/github.com/CodisLabs/codis $CODIS
