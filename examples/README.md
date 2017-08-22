# Example files

This provides an example of using the cluster proportional vertical autoscaler
to vertically scale the resources for an nginx server poportional to the size 
of the cluster, in this case number of nodes.

Use below commands to create / delete one of the example:
```
kubectl create -f cpvpa-nginx-example.yaml
...
kubectl delete -f cpvpa-nginx-example.yaml
```
# RBAC configurations

RBAC authentication has been enabled by default in Kubernetes 1.6+. You will need
to create the following RBAC resources to give the controller the permissions to
function correctly.

Use below commands to create / delete the RBAC resources:
```
kubectl create -f RBAC-configs.yaml
...
kubectl delete -f RBAC-configs.yaml
```

RBAC documentation: http://kubernetes.io/docs/admin/authorization/
