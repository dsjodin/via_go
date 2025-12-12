curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/hosts \
  --insecure \
  --request POST \
  --data '{ "domain": "sfo.rainpole.io", "group_id": 1, "pool_id": 1, "hostname": "sfo01-m01-esx01", "ip": "172.16.60.101", "mac": "00:50:56:8a:7c:09", "reimage": true }' | jq

  curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/hosts \
  --insecure \
  --request POST \
  --data '{ "domain": "sfo.rainpole.io", "group_id": 1, "pool_id": 1, "hostname": "sfo01-m01-esx02", "ip": "172.16.60.2", "mac": "00:50:56:8a:7c:10", "reimage": true }' | jq

curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/hosts \
  --insecure \
  --request POST \
  --data '{ "domain": "sfo.rainpole.io", "group_id": 2, "pool_id": 2, "hostname": "sfo01-w01-esx01", "ip": "172.16.61.101", "mac": "00:50:56:8a:73:65", "reimage": true }' | jq

  curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/hosts \
  --insecure \
  --request POST \
  --data '{ "domain": "sfo.rainpole.io", "group_id": 3, "pool_id": 3, "hostname": "sfo01-m01-hw-efi", "ip": "172.16.4.10", "mac": "e4:43:4b:2f:a5:20", "reimage": true }' | jq

  curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/hosts \
  --insecure \
  --request POST \
  --data '{ "domain": "sfo.rainpole.io", "group_id": 2, "pool_id": 1, "hostname": "sfo01-m01-vm-efi", "ip": "172.16.60.102", "mac": "00:50:56:8a:2a:b6", "reimage": true }' | jq