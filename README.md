# koob OS

**koob OS** is a minimal, immutable Kubernetes distribution built from scratch using the Linux kernel and a Go-based user space. It removes the standard Linux user space (Systemd, GNU Coreutils, package managers) and replaces them with a custom Go-based init system (`koobd`) and shell utilities derived from `u-root`.

## Core features
- **Security-first architecture**:
  - **Unified Kernel Image (UKI)**: Single signed EFI binary containing the kernel, initramfs, and root filesystem.
  - **UEFI Secure Boot**: Custom key enrollment with self-generated certificates (PK, KEK, db) and `.auth` files.
  - **Immutable root**: Read-only SquashFS file system with OverlayFS for runtime volatility.
- **Kubernetes v1.36.0**: Full control plane (API, scheduler, controller manager, etcd) and Kubelet.
- **Modern runtime**: Containerd 2.0.
- **Custom Go stack**: Uses `koobd` as PID 1 for initialization and `koobadm` for PKI and bootstrap.
- **Resource efficient**: 89MB ISO size.
- **Open source**: Apache License 2.0.

## Quick start
**koob OS** is designed to be built from source. This ensures that you own the entire trust chain (Secure Boot keys) and can customize the kernel command line for your specific hardware.

This process is tested on **Debian 12 and 13**.

### 1. Build the koob OS ISO
```bash
git clone https://github.com/koob-foo/koob-os.git && cd koob-os

# Optional: Adjust console settings for bare-metal hardware
# Edit config/cmdline.txt (e.g., change console=ttyS0 to console=tty0)

# Note: 'make all' runs 'hack/setup-environment.sh' which uses sudo to install dependencies.
# You may be prompted for your password.

make all

# Note: Compile time including Linux kernel and GLIBC is approximately 30 minutes.

```

### 2. Deploy the control plane
```bash
# Launch the VM
# Note: This script requires sudo privileges to interact with libvirt.
# You may be prompted for your password.
./examples/libvirt/deploy-node.sh koob-control-plane

# Connect to the serial console
sudo virsh console koob-control-plane

# Inside the VM: Bootstrap the cluster
koobadm init

# Inside the VM: Get the admin config for your host
koobadm config
```

### 3. Deploy a worker node
```bash
# On the Control Plane: Generate the join command
koobadm token create --print-join-command

# On the Host: Launch a worker VM
# Note: This script requires sudo privileges to interact with libvirt.
# You may be prompted for your password.
./examples/libvirt/deploy-node.sh koob-worker-1

# Connect to the worker console
sudo virsh console koob-worker-1

# Inside the Worker: Run the join command from the control plane
koobadm join <control-plane-ip>:6443 --token <token> ...
```

For more detailed instructions, see the [full libvirt deployment guide](examples/libvirt/README.md).

## Join the conversation
If you have questions or feedback, or if you want to dive deeper into the technical details, check out [GitHub Discussions](https://github.com/koob-foo/koob-os/discussions).
