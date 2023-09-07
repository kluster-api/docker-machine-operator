#!/bin/bash

# Copyright AppsCode Inc. and Contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

sudo su

#this update process is going to affect another update process defined in the docker installation
#for this reason we are using sleep after updating
apt-get -y update
sleep 30s

cd /root

# <UDF name="aws_access_key_id" label="AWS Access Key id" />
# <UDF name="aws_secret_access_key" label="AWS Secret Access key" />
# <UDF name="aws_node_machine_type" label="AWS Machine type (default: t3.medium)" />
# <UDF name="worker_node_count" label="Worker nodes in cluster (default: 1)" />
# <UDF name="cluster_name" label="AWS Kubernetes Cluster name (default: capa-cluster)" />
# <UDF name="github_token" label="Github public repo access token" />
# <UDF name="aws_region" label="AWS region. (default: us-east-2)" />

# <UDF name="cluster_k8s_version" label="kubernetes version of the cluster. (default: v1.22.9)" />
# <UDF name="vpc_cidr" label="CIDR range of AWS VPC. (default: 10.0.0.0/16)" />

# for logs....

# <UDF name="nats_creds" label="NATS Creds" />
# <UDF name="nats_server" label="NATS Server" />
# <UDF name="shipper_subject" label="Shipper NATS subject" />

export AWS_ACCESS_KEY_ID="AKIAWSYRJ2YDXU7FK4CF"
export AWS_SECRET_ACCESS_KEY="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"
export AWS_NODE_MACHINE_TYPE="t3.medium"
export WORKER_NODE_COUNT="3"
export CLUSTER_NAME="c2"
export AWS_REGION="us-east-1"
export CLUSTER_K8S_VERSION="v1.27.0"
export VPC_CIDR="10.1.0.0/16"
export GITHUB_TOKEN="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"
export NATS_CREDS="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"
export NATS_SERVER="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"
export SHIPPER_SUBJECT="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"

set -xeou pipefail
exec >/root/stackscript.log 2>&1

delete_roles() {
    aws iam delete-role --role-name "control-plane"${CLUSTER_NAME}"-kubedb-managed" || true
    aws iam delete-role --role-name "controllers"${CLUSTER_NAME}"-kubedb-managed" || true
    aws iam delete-role --role-name "nodes"${CLUSTER_NAME}"-kubedb-managed" || true

    CONTROLPLANE_ROLE="controlplane-"${CLUSTER_NAME}"-kubedb-managed"
    aws iam detach-role-policy --role-name ${CONTROLPLANE_ROLE} --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy || true
    aws iam delete-role --role-name "controlplane-"${CLUSTER_NAME}"-kubedb-managed" || true
}

# http://redsymbol.net/articles/bash-exit-traps/
# https://unix.stackexchange.com/a/308209
rollback() {
    kubectl delete cluster $CLUSTER_NAME -n $cluster_namespace || true
    delete_roles
    cat >bootstrap-config.yaml <<EOF
    apiVersion: bootstrap.aws.infrastructure.cluster.x-k8s.io/v1beta1
    kind: AWSIAMConfiguration
    spec:
      stackName: $CLUSTER_NAME-$cluster_namespace
      eks:
        iamRoleCreation: false
        managedMachinePool:
          disable: false
        fargate:
          disable: false
EOF
    clusterawsadm bootstrap iam delete-cloudformation-stack --config bootstrap-config.yaml
}

function finish {
    result=$?
    if [ $result -ne 0 ]; then
        rollback || true
    fi

    if [ $result -ne 0 ]; then
        echo "Cluster provision: Task failed !"
    else
        echo "Cluster provision: Task completed successfully !"
    fi

    sleep 5s

    [ ! -f /root/result.txt ] && echo $result >/root/result.txt
}
trap finish EXIT

#curl -fsSLO https://github.com/bytebuilders/nats-logger/releases/latest/download/nats-logger-linux-amd64.tar.gz
#tar -xzvf nats-logger-linux-amd64.tar.gz
#chmod +x nats-logger-linux-amd64
#mv nats-logger-linux-amd64 nats-logger

#SHIPPER_FILE=/root/stackscript.log ./nats-logger &

#...

export_variables() {
    export AWS_SSH_KEY_NAME=""
}
export_variables

kind_version=v0.19.0
k8s_version=v1.23.17
clusterctl_version=v1.2.4
clusterawsadm_version=v1.5.0
infrastructure_version=aws:v2.2.0
iam_authenticator_version=0.5.9
cluster_namespace=kubedb-managed
HOME="/root"

#for IRSA
EKS_CLUSTER_NAME=$cluster_namespace"_"$CLUSTER_NAME"-control-plane"
EDNS_SA="external-dns-operator"
EDNS_NS="kubeops"
CONTROLPLANE_ROLE="controlplane"-$CLUSTER_NAME-$cluster_namespace

#architecture
case $(uname -m) in
    x86_64)
        sys_arch=amd64
        ;;
    arm64 | aarch64)
        sys_arch=arm64
        ;;
    ppc64le)
        sys_arch=ppc64le
        ;;
    s390x)
        sys_arch=s390x
        ;;
    *)
        sys_arch=amd64
        ;;
esac

#opearating system
opsys=windows
if [[ "$OSTYPE" == linux* ]]; then
    opsys=linux
elif [[ "$OSTYPE" == darwin* ]]; then
    opsys=darwin
fi

timestamp() {
    date +"%Y/%m/%d %T"
}

log() {
    local type="$1"
    local msg="$2"
    local script_name=${0##*/}
    echo "$(timestamp) [$script_name] [$type] $msg"
}

retry() {
    local retries="$1"
    shift

    local count=0
    local wait=5
    until "$@"; do
        exit="$?"
        if [ $count -lt $retries ]; then
            log "INFO" "Attempt $count/$retries. Command exited with exit_code: $exit. Retrying after $wait seconds..."
            sleep $wait
        else
            log "INFO" "Command failed in all $retries attempts with exit_code: $exit. Stopping trying any further...."
            return $exit
        fi
        count=$(($count + 1))
    done
    return 0
}

#download docker from: https://docs.docker.com/engine/install/ubuntu/
install_docker_apt() {
    echo "--------------installing docker------------------"
    #remove all conflicting packages
    for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do sudo apt-get remove $pkg || true; done
    apt -y install docker.io
}

install_yq() {
    snap install yq
}

#install kind from: https://kind.sigs.k8s.io/docs/user/quick-start/#installing-from-source
install_kind() {
    echo "--------------installing kind--------------"

    local cmnd="curl -Lo ./kind https://kind.sigs.k8s.io/dl/${kind_version}/kind-linux-${sys_arch}"
    retry 5 ${cmnd}

    chmod +x ./kind

    cmnd="mv ./kind /usr/local/bin/kind"
    retry 5 ${cmnd}
}

create_kind_cluster() {
    #create cluster
    cmnd="kind delete cluster"
    retry 5 ${cmnd}

    sleep 5s

    kind create cluster --image=kindest/node:${k8s_version}
    #    kind create cluster --image=kindest/node:latest
    kubectl wait --for=condition=ready pods --all -A --timeout=5m
}

#download kubectl from: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
install_kubectl() {
    echo "--------------installing kubectl--------------"
    ltral="https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/${opsys}/${sys_arch}/kubectl"
    local cmnd="curl -LO"
    retry 5 ${cmnd} ${ltral}

    ltral="https://dl.k8s.io/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/${opsys}/${sys_arch}/kubectl.sha256"
    cmnd="curl -LO"
    retry 5 ${cmnd} ${ltral}

    echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check

    cmnd="install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl"
    retry 5 ${cmnd}
}

#download clusterctl from: https://cluster-api.sigs.k8s.io/user/quick-start.html
install_clusterctl() {
    local cmnd="curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/${clusterctl_version}/clusterctl-${opsys}-${sys_arch} -o clusterctl"
    retry 5 ${cmnd}

    cmnd="install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl"
    retry 5 ${cmnd}

    clusterctl version
}

install_clusterawsadm() {
    echo "--------------installing clusterawsadm--------------"

    local cmnd="curl -L https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/download/${clusterawsadm_version}/clusterawsadm-${opsys}-${sys_arch} -o clusterawsadm"
    retry 5 ${cmnd}

    chmod +x clusterawsadm
    mv clusterawsadm /usr/local/bin
    clusterawsadm version
}

#download helm from apt (debian/ubuntu) https://helm.sh/docs/intro/install/
install_helm() {
    echo "--------------installing helm------------------"
    curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg >/dev/null
    apt-get install apt-transport-https --yes
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list
    apt-get update
    apt-get install helm
}

#download aws-iam-authenticator from https://docs.aws.amazon.com/eks/latest/userguide/install-aws-iam-authenticator.html
install_aws_iam_authenticator() {
    local cmnd="curl -Lo aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v${iam_authenticator_version}/aws-iam-authenticator_${iam_authenticator_version}_${opsys}_${sys_arch}"

    retry 5 ${cmnd}
    chmod +x $HOME/aws-iam-authenticator
    mkdir -p $HOME/bin && cp ./aws-iam-authenticator $HOME/bin/aws-iam-authenticator && export PATH=$PATH:$HOME/bin
    echo 'export PATH=$PATH:$HOME/bin' >>~/.bashrc
    aws-iam-authenticator help
}

init_aws_infrastructure() {
    cat >bootstrap-config.yaml <<EOF
  apiVersion: bootstrap.aws.infrastructure.cluster.x-k8s.io/v1beta1
  kind: AWSIAMConfiguration
  spec:
    stackName: $CLUSTER_NAME-$cluster_namespace
    nameSuffix: $CLUSTER_NAME-$cluster_namespace
    eks:
      defaultControlPlaneRole:
        disable: true
      managedMachinePool:
        disable: true
      fargate:
        disable: true
EOF
    clusterawsadm bootstrap iam create-cloudformation-stack --config bootstrap-config.yaml

    cat <<-EOF >policy.json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": [
                    "ec2.amazonaws.com",
                    "eks.amazonaws.com"
                ]
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
EOF
    aws iam create-role --role-name ${CONTROLPLANE_ROLE} --assume-role-policy-document file://policy.json
    aws iam attach-role-policy --role-name ${CONTROLPLANE_ROLE} --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy

    export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)
    export EXP_MACHINE_POOL=true

    local cmnd="clusterctl init --infrastructure ${infrastructure_version}"
    retry 5 ${cmnd}

    echo "waiting for pods to be ready"
    kubectl wait --for=condition=Ready pods -A --all --timeout=10m
}

install_capi_config() {
    curl -fsSLO https://github.com/bytebuilders/capi-config/releases/download/v0.0.1/capi-config-linux-amd64.tar.gz
    tar -xzf capi-config-linux-amd64.tar.gz
    cp capi-config-linux-amd64 /bin
}

configure_capa() {
    install_capi_config
    cat /root/cluster.yaml >/root/cluster1.yaml
    capi-config-linux-amd64 capa --vpc-cidr="${VPC_CIDR}" --min-node-count=2 --max-node-count=6 --managedcp-role=${CONTROLPLANE_ROLE} --managedmp-role="nodes${CLUSTER_NAME}-${cluster_namespace}" </root/cluster1.yaml >/root/cluster.yaml
}

create_eks_cluster() {
    echo "Creating eks Cluster"

    cmnd="clusterctl generate cluster"

    kubectl create ns $cluster_namespace

    retry 5 ${cmnd} ${CLUSTER_NAME} --flavor eks-managedmachinepool --kubernetes-version ${CLUSTER_K8S_VERSION} --worker-machine-count=${WORKER_NODE_COUNT} -n $cluster_namespace >/root/cluster.yaml

    configure_capa

    kubectl apply -f /root/cluster.yaml

    echo "creating cluster..."
    kubectl wait --for=condition=ready cluster --all -A --timeout=30m
    kubectl get secret ${CLUSTER_NAME}-user-kubeconfig --template={{.data.value}} -n $cluster_namespace | base64 -d >${HOME}/kubeconfig
}

install_aws_cli() {
    echo "installing aws cli..."
    apt install unzip >/dev/null
    curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" >/dev/null
    unzip awscliv2.zip >/dev/null
    sudo ./aws/install >/dev/null
}

pivot_cluster() {
    local cmnd="clusterctl init --infrastructure ${infrastructure_version} --kubeconfig=${HOME}/kubeconfig"
    retry 5 ${cmnd}
    kubectl wait --kubeconfig=${HOME}/kubeconfig --for=condition=ready pods --all -A --timeout=10m

    clusterctl move --to-kubeconfig=${HOME}/kubeconfig -n $cluster_namespace
}

create_credential_secret() {
    export KUBECONFIG=${HOME}/kubeconfig

    cat <<EOF | kubectl apply -f -
  apiVersion: v1
  kind: Secret
  metadata:
    name: aws-credential
    namespace: ${cluster_namespace}
  type: Opaque
  stringData:
    credentials: |
      {
        "access_key": $AWS_ACCESS_KEY_ID,
        "secret_key": $AWS_SECRET_ACCESS_KEY
      }
EOF
}

init() {
    install_docker_apt
    install_yq
    install_kind
    install_kubectl

    sleep 1m

    create_kind_cluster
    install_clusterctl
    install_clusterawsadm
    install_helm
    install_aws_cli
    install_aws_iam_authenticator
    init_aws_infrastructure
    #    create_eks_cluster
    #    pivot_cluster
    #    create_credential_secret
}
init
