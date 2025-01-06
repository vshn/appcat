FROM mcr.microsoft.com/devcontainers/go:dev-1.23-bookworm

ARG TARGETOS
ARG TARGETARCH

ENV HELMVERS="v3.16.4"
ENV YQVERSION="v4.44.6"
ENV KUBECOLORVERSION="0.4.0"

# Install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/${TARGETARCH}/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin

# Install helm
RUN bash -c "curl -s https://get.helm.sh/helm-${HELMVERS}-linux-${TARGETARCH}.tar.gz > helm3.tar.gz" && tar -zxvf helm3.tar.gz linux-${TARGETARCH}/helm && chmod +x linux-${TARGETARCH}/helm && mv linux-${TARGETARCH}/helm /usr/local/bin && rm helm3.tar.gz && rm -R linux-${TARGETARCH}

# Install yq
RUN curl -L -o yq "https://github.com/mikefarah/yq/releases/download/${YQVERSION}/yq_linux_${TARGETARCH}" && install -c -m 0755 yq /usr/local/bin

# Install kubecolor
RUN curl -L -o kubecolor.tar.gz https://github.com/kubecolor/kubecolor/releases/download/v${KUBECOLORVERSION}/kubecolor_${KUBECOLORVERSION}_linux_${TARGETARCH}.tar.gz && tar -xvzf kubecolor.tar.gz && install -c -m 0755 kubecolor /usr/local/bin

# Install bash-complete to make kubectl autocomplete work
RUN apt-get update && apt-get install bash-completion make vim -y && rm -rf /var/lib/apt/lists/*
