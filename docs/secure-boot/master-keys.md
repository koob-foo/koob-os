# Local Secure Boot Master Keys

As a **Build-it-Yourself** distribution, **koob OS** ensures you own your trust chain. You are the "Master" of your keys: they are generated locally, held privately, and never uploaded **anywhere**.

## 1. Local Generation

Run the following commands on your local development machine to generate your unique master keys.

```bash
# Ensure dependencies are installed (e.g., sbsigntool, efitools, uuid-runtime)
make setup

# Generate the keys
make keys
```

The keys will be located in `tmp/secure-boot/keys/`.

## 2. Key Persistence

By default, the keys in `tmp/` are transient and will be deleted if you run `make distclean`. To establish a permanent trust chain:

1.  **Move keys to your home directory**:
    ```bash
    cp -r tmp/secure-boot/keys ~/.koob/
    ```
2.  **Stable Identity**: Subsequent runs of `make keys` will detect `~/.koob/keys/` and reuse your existing identity instead of generating new ones.

This ensures that any OS image you build today or next year will be trusted by your hardware without requiring you to re-enroll keys in the BIOS.

## 3. Secure Backup

> [!WARNING]
> **Extremely Important**: Copy your `~/.koob/keys/` directory to a secure, offline location (like an encrypted USB drive). If you lose these keys, you will be unable to sign future UKIs that your hardware trusts.

## 4. Why no GitHub Secrets?

We have explicitly removed support for "Master Keys" in CI/CD (GitHub Secrets) for the following reasons:
1.  **Security**: Your private keys never leave your machine. They cannot be leaked via CI logs or compromised GitHub accounts.
2.  **Trust**: You don't have to trust "Official Builds" from a third party. You verify the source code, build the "raw materials" in CI if you wish, but the final **signing** always happens under your direct control.
3.  **Customization**: Since you sign locally, you can modify `config/cmdline.txt` (e.g., for bare-metal console access) and sign the result instantly.
