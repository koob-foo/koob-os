# koob OS libvirt deployment guide

This guide explains how to deploy **koob OS** using the provided libvirt examples and bootstrap a Kubernetes cluster.

> [!IMPORTANT]
> This guide assumes you have built the **koob OS** ISO yourself. If you haven't done so yet, please follow the [Build Guide](../../docs/build.md) first. This ensures you own the Secure Boot trust chain and have configured the correct console settings for your environment.

Deploy **koob OS** to libvirt in one of the following modes.

> [!NOTE]
> These scripts use `sudo` to interact with libvirt and format enrollment disks. You may be prompted for your password.

### UEFI mode (setup mode)
This is the default and recommended mode.
```bash
./deploy-node.sh koob-control-plane
```
The VM boots in UEFI **setup mode**. In this state, Secure Boot support is active but not yet enforced. This allows your signed UKI to boot without manual BIOS steps while you iterate.

To require strict enforcement, [verify the boot integrity](../../docs/secure-boot/verification.md) and [enroll your keys](../../docs/secure-boot/enrollment.md). This transitions the firmware to **user mode** and begins strict enforcement.

### Insecure mode (legacy BIOS)
Use this mode to bypass Secure Boot for testing.
```bash
./deploy-node.sh koob-control-plane --insecure
```
The VM boots using legacy BIOS/MBR. This mode disables Secure Boot entirely.

## Node deployment

Run the deployment script to create or replace VMs in libvirt using the built ISO.

### 1. Deploy the control plane
```bash
./deploy-node.sh koob-control-plane
```

### 2. Deploy a worker node
```bash
./deploy-node.sh koob-worker-1
```

### 3. Connect to the console
Once the VM is running, connect to its serial console:
```bash
# For the control plane:
sudo virsh console koob-control-plane

# For the worker:
sudo virsh console koob-worker-1
```
*(Escape character is `^]` (Ctrl+])*

## Usage

After connecting to the console, the `>` prompt appears.

### 1. Bootstrap the control plane
Run the initialization command to generate the PKI, static pod manifests, and KubeConfig.
```bash
koobadm init
```

### 2. Verify components
Check processes or logs to ensure services are running correctly:
```bash
ps
cat /var/log/kubelet.log
```

### 3. Access from the host
The network uses a bridge via libvirt (`virbr0`). Ping the VM address shown at boot to verify connectivity. To control the cluster from your host, use the `admin.conf` file.

**In the VM:**
```bash
koobadm config
```
This command prints the `admin.conf` as a base64-encoded block. Copy the block.

**On the host:**
```bash
cat > admin.b64 <<EOF
<PASTE BLOCK HERE>
EOF

base64 -d admin.b64 > admin.conf
export KUBECONFIG=$(pwd)/admin.conf
kubectl get nodes
```

### 4. Add worker nodes
After deploying a worker VM, join it to the cluster.

**Generate the join command on the control plane:**
```bash
koobadm token create --print-join-command
```
This command prints a ready-to-use `koobadm join` command containing the token and CA hash.

**Execute the join on the worker node:**
Paste and run the command generated in the previous step:
```bash
koobadm join <control-plane-ip>:6443 --token <token> --discovery-token-ca-cert-hash <hash>
```

## Container networking

**koob OS** is CNI-agnostic and works with any Kubernetes-compatible Container Network Interface (CNI), such as Cilium, Calico, or Flannel.

### Install Cilium (example)
For demonstration purposes, this guide uses **Cilium** for eBPF-based networking and observability.

```bash
export CONTROL_PLANE_IP="x.x.x.x"
helm install \
    cilium \
    cilium/cilium \
    --version 1.18.5 \
    --namespace kube-system \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=true \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup \
    --set k8sServiceHost=$CONTROL_PLANE_IP \
    --set k8sServicePort=6443
```

> [!NOTE]
> Cilium defaults to two operator replicas for high availability (HA). On a single-node cluster, one replica remains `Pending` until you add a second node.

### Verify the installation
```bash
kubectl get pods -n kube-system -l k8s-app=cilium
cilium status --wait
```
