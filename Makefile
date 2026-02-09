# koob OS Minimal Build System
# Usage: make all

# ANSI Colors
OK_COLOR    = \033[1;32m
INFO_COLOR  = \033[1;36m
WARN_COLOR  = \033[1;33m
BUILD_COLOR = \033[1;36m
NC          = \033[0m

# Versioning
VERSION ?= $(shell cat VERSION)
export VERSION

.PHONY: all setup fetch keys kernel binaries iso clean distclean help

# --- Primary Build Chain ---
all: setup fetch keys kernel binaries iso
	@printf "$(OK_COLOR)[ SUCCESS ]$(NC) koob OS build complete.\n"

setup:
	@printf "$(INFO_COLOR)[ SETUP ]$(NC) Initializing environment...\n"
	@sh hack/setup-environment.sh

fetch:
	@printf "$(INFO_COLOR)[ FETCH ]$(NC) Downloading external binaries...\n"
	@sh hack/fetch-deps.sh

keys:
	@printf "$(INFO_COLOR)[ KEYS ]$(NC) Generating signing keys...\n"
	@sh hack/gen-sb-keys.sh

kernel:
	@printf "$(BUILD_COLOR)[ BUILD ]$(NC) Compiling Linux kernel...\n"
	@sh hack/build-kernel.sh

binaries:
	@printf "$(BUILD_COLOR)[ BUILD ]$(NC) Compiling system binaries...\n"
	@sh hack/build-koobd.sh
	@sh hack/build-koobadm.sh
	@sh hack/build-bb.sh
	@sh hack/build-iptables.sh
	@sh hack/build-libs.sh

iso:
	@printf "$(BUILD_COLOR)[ BUILD ]$(NC) Generating rootfs and ISO...\n"
	@sh hack/build-squashfs.sh
	@sh hack/build-initramfs.sh
	@sh hack/build-iso.sh

pack: setup keys
	@printf "$(BUILD_COLOR)[ BUILD ]$(NC) Packaging ISO from existing artifacts (bzImage/initramfs)...\n"
	@sh hack/build-iso.sh

# --- Testing & Validation ---

test:
	@printf "$(INFO_COLOR)[ TEST ]$(NC) Running Go unit tests...\n"
	@go test -v ./pkg/...

verify:
	@printf "$(INFO_COLOR)[ VERIFY ]$(NC) Running artifact validation...\n"
	@sh hack/test-artifacts.sh

e2e:
	@printf "$(INFO_COLOR)[ E2E ]$(NC) Running boot smoke test...\n"
	@sh hack/test-boot.sh

full-test: test verify e2e
	@printf "$(OK_COLOR)[ SUCCESS ]$(NC) All tests passed.\n"

# --- Utilities ---

clean:
	@printf "$(INFO_COLOR)[ CLEAN ]$(NC) Cleaning artifacts...\n"
	@rm -f *.squashfs *.cpio *.efi koob-os.iso
	@rm -rf tmp/iso_root

distclean: clean
	@printf "$(WARN_COLOR)[ RESET ]$(NC) Full system reset...\n"
	@rm -rf tmp/ bzImage

help:
	@echo "koob OS Build System"
	@echo ""
	@echo "Targets:"
	@echo "  all        Full build sequence"
	@echo "  setup      Install dependencies"
	@echo "  fetch      Get external binaries"
	@echo "  keys       Gen Secure Boot keys"
	@echo "  kernel     Build Linux kernel"
	@echo "  binaries   Build Go/C utilities"
	@echo "  iso        Build final image (full chain)"
	@echo "  pack       Build ISO from existing bzImage/initramfs"
	@echo "  clean      Remove build artifacts"
	@echo "  distclean  Full system reset"
