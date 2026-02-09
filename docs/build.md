# koob OS build guide

This guide provides instructions to set up your environment and build the **koob OS** image using the integrated `make` environment.

## Directory Structure
- `cmd/`: Command line utilities and system components (e.g., `koobd`, `koobadm`).
- `pkg/`: Core logic and shared Go packages.
- `config/`: Configuration files for the kernel, containerd, and system boot.
- `hack/`: Build, initialization, and automation scripts.
- `docs/`: Technical guides and architecture documentation.
- `examples/`: Deployment scripts and virtualization examples.

## Build Environment

To build the entire operating system (kernel, user space, and ISO), you can simply run `make all`. This process takes approximately **30 minutes** on a modern machine, as it compiles the Linux kernel and GLIBC from source.

If you prefer a more detailed explanation of the build process, see the [Step-by-Step Build](#step-by-step-build) section.

If you just want to get started fast and are not concerned about a verified ISO, see the [Fast Track Build](#fast-track-build) section.

## Step-by-Step Build

### 1. Set up the environment
Prepare your build environment and install required dependencies:

```bash
# Note: 'make setup' runs 'hack/setup-environment.sh' which uses sudo to install dependencies.
# You may be prompted for your password.

make setup
```

The `make setup` command does the following:
- Installs required build dependencies (gcc, make, squashfs-tools, libvirt, etc.).
- Checks for a Go installation (Go 1.21 or later is required).
- Ensures helper scripts in the `hack/` directory are executable.

**Note on Go:** If you need to install or update Go, see the [official installation guide](https://go.dev/doc/install).

### 2. Prepare dependencies
Download the Kubernetes binaries and core utilities:
```bash
make fetch
```

> **Note**: This process prepares the directory structure and fetches the necessary pre-compiled binaries to populate the root filesystem.

### 3. Secure boot prerequisites
Generate your custom "koob root" keys and prepare the enrollment files:
```bash
make keys
```

#### Persistent Keys (Optional)
By default, keys are stored in `tmp/secure-boot/keys/` and are lost if you run `make distclean`. To persist your trust chain across builds:
1.  Copy your generated keys: `cp -r tmp/secure-boot/keys ~/.koob/`
2.  Subsequent runs of `make keys` will automatically detect and use the keys in your home directory.

## Hardware Customization

Because **koob OS** is a "Build-it-Yourself" distribution, you can customize the operating system for your specific hardware before generating the ISO.

### Customizing the Kernel Command Line
The kernel command line determines how the OS boots and which console it uses. You can modify this in `config/cmdline.txt`.

*   **Virtual Machines (Libvirt/QEMU)**: Use `console=ttyS0` to see the shell via `virsh console`.
*   **Bare Metal (Monitor/Keyboard)**: Use `console=tty0` to use the physical display and keyboard.
*   **Bare Metal (Serial)**: Use `console=ttyS0,115200` if your server has a physical COM port.

> [!IMPORTANT]
> The **last** `console=` argument in the string becomes the primary console for the interactive shell.

---

## Build process

**koob OS** uses a **monolithic architecture** where the entire root file system is embedded into the signed initramfs. This ensures 100% Secure Boot integrity for every byte of the operating system.

## Build the custom Linux kernel
The **koob OS** kernel is a minimal, hardened build optimized for Kubernetes workloads. To compile the kernel from source and prepare the `bzImage`:

```bash
make kernel
```
This command uses pre-configured kernel settings to ensure compatibility with the UKI and the initramfs boot process.

## Build the system binaries
Compile the custom Go-based init system, bootstrap utilities, and hermetic libraries (including GLIBC):

```bash
make binaries
```
This builds the following components:
- `koobd`: The PID 1 init system.
- `koobadm`: The cluster bootstrap and PKI management tool.
- `bb`: The minimal BusyBox-like multi-call binary.
- **Hermetic libraries**: Compiles GLIBC and other shared libraries.
- System-level iptables.

> [!NOTE]
> You can edit `hack/build-bb.sh` to include additional Linux shell utility commands in the system's multi-call binary.

### Build the OS components
To build the complete system, including the root filesystem and the final signed UKI ISO, run:

```bash
make iso
```

This command executes the following steps in order:
1. **Build the rootfs**: Creates the SquashFS image containing the Kubernetes binaries and user space.
2. **Build the initramfs**: Bundles the rootfs into the CPIO archive.
3. **Build the UKI**: Combines the kernel, initramfs, and cmdline into a single signed EFI binary.
4. **Build the ISO**: Packages the UKI into a bootable ISO.

### Granular build commands
If you need to build specific components, use the following commands:

| Command | Description |
| :--- | :--- |
| `make rootfs` | Build the SquashFS root filesystem image. |
| `make initramfs` | Build the initramfs CPIO archive. |
| `make uki` | Build the signed Unified Kernel Image (.efi). |
| `make iso` | Build the final bootable ISO. |

> [!TIP]
> **Kernel customization**: The **koob OS** build process compiles a hardened Linux kernel optimized for Kubernetes. You can modify the kernel configuration in the `kernel/` directory and rebuild it using `make kernel`.

### Cleanup
To restore the repository to a fresh state, run:
```bash
make clean         # Remove temporary build artifacts (squashfs, ISO, etc.)
make distclean     # Remove EVERYTHING including the kernel and keys
```

> **Warning**: Deleting `secure-boot/` after enrolling keys in UEFI breaks your ability to sign new UKIs that the firmware trusts. Only use `distclean` if you are starting fresh.

## Fast Track Build

If you want to skip the 30+ minute compilation process and go straight to assembly and signing, you can use pre-compiled building blocks from the [latest release](https://github.com/koob-foo/koob-os/releases).

### 1. Download Artifacts
Download the following files from a GitHub Release:
*   `bzImage` (The Linux Kernel)
*   `initramfs.cpio` (The compressed root filesystem)

> [!WARNING]
> Released artifacts are **unsigned**. They cannot be cryptographically verified and should only be used for testing purposes. To generate verifiable, secure artifacts, you must build them locally from source.

### 2. Place in Repository Root
Move both files into the root of your `koob-os` repository:
```bash
mv ~/Downloads/bzImage .
mv ~/Downloads/initramfs.cpio .
```

### 3. Assemble and Sign
Run the specialized packaging command:
```bash
make pack
```

The `make pack` command is specifically designed for this workflow. It skips all compilation, compression, and rootfs generation steps, proceeding directly to:
1.  **UKI Assembly**: Embedding your `config/cmdline.txt` and generating the `.efi` binary.
2.  **Secure Boot Signing**: Signing the UKI with your local keys (from `~/.koob/keys/` or `tmp/`).
3.  **ISO Generation**: Creating the final bootable image.

> [!TIP]
> This is the fastest way to test kernel command-line changes (`config/cmdline.txt`) or Secure Boot key updates without rebuilding the entire OS.

---
