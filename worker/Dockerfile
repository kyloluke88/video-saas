FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    bzip2 \
    ca-certificates \
    curl \
    ffmpeg \
    fontconfig \
    fonts-dejavu \
    fonts-noto-cjk \
    git \
    && rm -rf /var/lib/apt/lists/*

# MFA's published dependency stack is much more stable on x86_64 than on
# linux-aarch64, so we install it through Miniforge/conda and run the worker
# container under linux/amd64 in docker-compose.
RUN arch="$(uname -m)" && \
    case "$arch" in \
      x86_64) installer="Miniforge3-Linux-x86_64.sh" ;; \
      aarch64|arm64) installer="Miniforge3-Linux-aarch64.sh" ;; \
      *) echo "unsupported architecture: $arch" && exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/conda-forge/miniforge/releases/latest/download/${installer}" -o /tmp/miniforge.sh && \
    bash /tmp/miniforge.sh -b -p /opt/conda && \
    rm -f /tmp/miniforge.sh && \
    /opt/conda/bin/conda config --system --set auto_update_conda false && \
    /opt/conda/bin/conda install -y -n base -c conda-forge mamba && \
    /opt/conda/bin/mamba create -y -p /opt/mfa-env -c conda-forge python=3.10 montreal-forced-aligner && \
    mkdir -p /opt/mfa && \
    useradd -ms /bin/bash mfauser && \
    chown -R mfauser /opt/mfa /opt/mfa-env && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa server init' && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa model download dictionary japanese_mfa' && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa model download acoustic japanese_mfa' && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa model download g2p japanese_mfa' && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa model download dictionary mandarin_china_mfa' && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa model download acoustic mandarin_mfa' && \
    su - mfauser -c 'export PATH=/opt/mfa-env/bin:$PATH; MFA_ROOT_DIR=/opt/mfa mfa model download g2p mandarin_china_mfa' && \
    /opt/conda/bin/conda clean -afy

ENV MFA_ROOT_DIR="/opt/mfa"
ENV PATH="/opt/mfa-env/bin:/opt/conda/bin:${PATH}"

WORKDIR /app

RUN go install github.com/air-verse/air@v1.61.7

COPY go.mod go.sum ./
RUN go mod download

COPY assets/fonts/ /usr/share/fonts/custom/
RUN fc-cache -f

COPY . .

CMD ["air"]
