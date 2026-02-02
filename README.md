# koob OS

> [!NOTE]
> **Status**: Final code review in progress.
> **v0.1.0 release**: February 9, 2026.

**koob OS** is a minimal, immutable Kubernetes distribution built from scratch. It uses a custom Linux kernel, the Containerd 2.0 runtime, and Kubernetes 1.35. The distribution replaces the standard Linux user space with a custom Go-based init system (`koobd`) and shell utilities derived from `u-root`.

### Purpose
**koob OS** provides the foundational building blocks for designing and creating your own minimal, secure Kubernetes operating systems. It serves as a hands-on reference to:
- **Boot fundamentals**: Understand the process of booting the Linux kernel, the initramfs, and PID 1 initialization.
- **UEFI Secure Boot cryptography**: Navigate the standards required for a trusted, hardware-verified boot chain.
- **Immutable root**: Implement immutable architectures using SquashFS for read-only compression and OverlayFS for runtime state management.
- **Cluster lifecycle**: Explore the internal setup and runtime of a Kubernetes cluster through a transparent, from-scratch implementation.

## Core features
- **Security-first architecture**:
  - **Unified Kernel Image (UKI)**: Single signed EFI binary containing the kernel, initramfs, and root filesystem.
  - **UEFI Secure Boot**: Custom key enrollment with self-generated certificates (PK, KEK, db) and `.auth` files.
  - **Immutable root**: Read-only SquashFS file system with OverlayFS for runtime volatility.
- **Kubernetes v1.35.0**: Full control plane (API, scheduler, controller manager, etcd) and Kubelet.
- **Modern runtime**: Containerd 2.0.
- **Custom Go stack**: Uses `koobd` as PID 1 for initialization and `koobadm` for PKI and bootstrap.
- **Resource efficient**: 91MB ISO size.
- **Open source**: Apache License 2.0.

## Quick start
Get a local cluster running on KVM in minutes. You can [download a pre-built ISO](https://github.com/koob-foo/koob-os/releases) or build it from source. This process is tested on **Debian 12 and 13**.

### 1. Build the koob OS ISO
```bash
git clone https://github.com/koob-foo/koob-os.git && cd koob-os
make all
```

### 2. Deploy the control plane
```bash
# Launch the VM
./examples/libvirt/deploy-libvirt-control-plane.sh

# Connect to the serial console
sudo virsh console koob-control-plane

# Inside the VM: Bootstrap the cluster
koobadm init

# Inside the VM: Get the admin config for your host
koobadm config
```

### 3. Deploy the worker node
```bash
# On the Control Plane: Generate the join command
koobadm token create --print-join-command

# On the Host: Launch a worker VM
./examples/libvirt/deploy-libvirt-worker.sh

# Connect to the worker console
sudo virsh console koob-worker

# Inside the Worker: Run the join command from the control plane
koobadm join <control-plane-ip>:6443 --token <token> ...
```

For more detailed instructions, see the [full libvirt deployment guide](examples/libvirt/README.md).

## Join the conversation
If you have questions or feedback, or if you want to dive deeper into the technical details, check out [GitHub Discussions](https://github.com/koob-foo/koob-os/discussions).
