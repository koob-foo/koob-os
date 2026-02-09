# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-09

### Summary
Koob OS is a minimal, immutable Kubernetes distribution built from scratch using the Linux kernel and a Go-based user space. It removes the standard Linux user space (Systemd, GNU Coreutils, package managers) and replaces them with a custom Go-based init system (koobd) and shell utilities derived from u-root.

koob OS 0.1.0 marks the initial public release.

> [!WARNING]
> Released artifacts are **unsigned**. They cannot be cryptographically verified and should only be used for testing purposes. To generate verifiable, secure artifacts, you must build them locally from source.
