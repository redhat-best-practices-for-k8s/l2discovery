#!/bin/sh

./build.sh
oc delete namespace l2discovery
oc apply -f resources/namespace.yml
oc apply -f resources/role.yml
oc apply -f resources/rolebinding.yml
oc apply -f resources/scc.yml
oc apply -f resources/service-account.yml
oc apply -f resources/daemonset.yml
