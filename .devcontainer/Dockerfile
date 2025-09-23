FROM ubuntu:24.04

ARG DEBIAN_FRONTEND=noninteractive

# Install essentials
RUN apt update && apt install -y curl git build-essential pkg-config libssl-dev

# Install Go (latest stable, e.g., 1.23 as of 2025)
RUN curl -LO https://go.dev/dl/go1.23.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.1.linux-amd64.tar.gz && \
    rm go1.23.1.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

# Install Go tools
RUN go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest && \
    go install golang.org/x/lint/golint@latest

# Install Rust (via rustup)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
ENV PATH=$PATH:/root/.cargo/bin

# Install WASM target for Rust apps
RUN rustup target add wasm32-unknown-unknown

# Install wasm-pack for Rust WASM development
RUN curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh

# Install Node.js (for React; LTS, e.g., 22.x)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt install -y nodejs

# Install global npm packages for React development
RUN npm install -g typescript eslint prettier

# Install Python and PIP
RUN apt-get update && \
    apt-get install -y python3 && \
    apt-get install -y python3 python3-pip && \
    rm -rf /var/lib/apt/lists/*

# Install Node.js 22.x LTS for React
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Set up workspace
WORKDIR /workspace