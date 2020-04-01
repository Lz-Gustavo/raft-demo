#!/bin/bash

sudo swapoff -a
sudo kubeadm init --pod-network-cidr=10.244.0.0/16

if [[! -f "$HOME/.kube" ]] then
    mkdir -p "$HOME/.kube"
fi

sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config | y
sudo chown $(id -u):$(id -g) $HOME/.kube/config

echo "deploying flannel..."
sudo kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml

# Remove master taint, which now enables pod scheduling on master node
kubectl taint nodes refrigerator node-role.kubernetes.io/master-
echo "finished!"