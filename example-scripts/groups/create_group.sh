#pxe-tftp-l2
curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/groups \
  --insecure \
  --request POST \
  --data '{ "image_id": 1, "name": "sfo-m01-l2pxe", "dns": "172.16.100.4,172.16.100.5", "ntp": "172.16.100.4,172.16.100.5", "netmask": "255.255.255.0", "password": "VMw@re1!", "gateway": "172.16.60.1", "syslog": "logs.sfo.rainpole.io", "bootmethod": "pxe" }' | jq

#pxe-tftp-l3
curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/groups \
  --insecure \
  --request POST \
  --data '{ "image_id": 1, "name": "sfo-w01-l3pxe", "dns": "172.16.100.4,172.16.100.5", "ntp": "172.16.100.4,172.16.100.5", "netmask": "255.255.255.0", "password": "VMw@re1!", "gateway": "172.16.61.1", "syslog": "logs.sfo.rainpole.io", "bootmethod": "pxe" }' | jq

#http-static-l2efi
curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/groups \
  --insecure \
  --request POST \
  --data '{ "image_id": 1, "name": "sfo-w01-l2efi", "dns": "172.16.100.4,172.16.100.5", "ntp": "172.16.100.4,172.16.100.5", "netmask": "255.255.255.0", "password": "VMw@re1!", "gateway": "172.16.60.1", "syslog": "logs.sfo.rainpole.io", "bootmethod": "https-dhcp" }' | jq

#http-dhcp-l3efi
  curl --header "Content-Type: application/json" \
  --user admin:VMware1! https://localhost:443/v1/groups \
  --insecure \
  --request POST \
  --data '{ "image_id": 1, "name": "sfo-w02-l3efi", "dns": "172.16.10.4,172.16.10.5", "ntp": "172.16.10.4,172.16.10.5", "netmask": "255.255.255.0", "password": "VMw@re1!", "gateway": "172.16.4.1", "syslog": "logs.sfo.rainpole.io", "bootmethod": "http-static" }' | jq
