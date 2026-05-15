# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.2] - 2026-05-15

### Summary
Koob OS 0.1.2 addresses critical security vulnerabilities and upgrades core system components.

- **Security Fix (CVE-2026-31431)**: Patched the "Copy Fail" container escape vulnerability by hardening the Linux kernel configuration.
- **Kernel Upgrade**: Updated Linux kernel to **v6.18.30**.
- **GLIBC Update**: Updated GLIBC to utilize the latest kernel headers.

> [!WARNING]
> Released artifacts are **unsigned**. They cannot be cryptographically verified and should only be used for testing purposes. To generate verifiable, secure artifacts, you must build them locally from source.

## [0.1.1] - 2026-05-11

### Summary
Koob OS 0.1.1 introduces minor infrastructure upgrades and build system refinements.

- **Upgraded Kubernetes** to v1.36.0.
- **Updated Cilium** to v1.19.3 in libvirt deployment examples.
- **Improved Build System**: Switched GLIBC mirrors.
- **Refined Documentation**: Updated deployment guides and README to reflect the new versions.

> [!WARNING]
> Released artifacts are **unsigned**. They cannot be cryptographically verified and should only be used for testing purposes. To generate verifiable, secure artifacts, you must build them locally from source.

## [0.1.0] - 2026-02-09

### Summary
Koob OS is a minimal, immutable Kubernetes distribution built from scratch using the Linux kernel and a Go-based user space. It removes the standard Linux user space (Systemd, GNU Coreutils, package managers) and replaces them with a custom Go-based init system (koobd) and shell utilities derived from u-root.

koob OS 0.1.0 marks the initial public release.

> [!WARNING]
> Released artifacts are **unsigned**. They cannot be cryptographically verified and should only be used for testing purposes. To generate verifiable, secure artifacts, you must build them locally from source.
