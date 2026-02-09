# Verify boot integrity

This guide explains how to verify the authenticity of a koob OS ISO and authorize its boot chain on your hardware.

## Prerequisites

Obtain the core building blocks from a release or your local build:
- `bzImage`: The Linux kernel.
- `initramfs.cpio`: The root filesystem.
- **Your local keys**: The `db.crt` certificate used to sign your UKI.

**Required tools**:
- `sbsigntool` (includes `sbverify`)
- `mtools` (includes `mcopy`)

> [!TIP]
> **Use with deployment scripts**: If you use the automated deployment scripts in `examples/libvirt/`, place the extracted files into `examples/libvirt/enroll/`. The scripts automatically package these files into a FAT-formatted virtual disk (`KOOB_KEYS`) for the UEFI firmware to read.

## Verify the ISO signature

Before booting, verify that the official koob OS master keys signed the Unified Kernel Image (UKI) on the ISO.

### 1. Locate your UKI
After running `make all`, your signed Unified Kernel Image is created as `koob-os.efi` (and also packaged inside `koob-os.iso`).

### 2. Verify with `sbverify`
Use your local `db.crt` (from `tmp/secure-boot/keys/` or `~/.koob/keys/`) to check the signature:

```bash
sbverify --cert path/to/your/db.crt koob-os.efi
```

**Success output**: `Signature verification OK`

## Next steps
After verifying the integrity of the image, proceed to the [Secure Boot enrollment guide](enrollment.md) to authorize the keys on your hardware.
