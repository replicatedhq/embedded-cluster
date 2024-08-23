ARG PLATFORM=linux/amd64

FROM --platform=$PLATFORM ubuntu:22.04 AS build

ARG DEBIAN_FRONTEND=noninteractive
ARG TZ=Etc/UTC

RUN apt-get update \
    && apt-get install -y \
    build-essential cmake libstdc++6 pkg-config unzip wget

RUN mkdir -p /fio
WORKDIR /fio

RUN wget -O- https://api.github.com/repos/axboe/fio/releases/latest \
    | grep "tarball_url" \
    | cut -d : -f 2,3 \
    | tr -d '", ' \
    | xargs -n1 wget -O fio.tar.gz -q
RUN tar -xzf fio.tar.gz --strip-components=1

RUN ./configure --build-static
RUN make -j$(nproc)

FROM ubuntu:22.04
COPY --from=build /fio/fio /output/fio
# ensure that the binary is statically linked
RUN ldd /output/fio 2>&1 | grep 'not a dynamic executable'
CMD [ "echo", "Done" ]
