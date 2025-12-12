curl -H "Content-Type: multipart/form-data" \
  --user admin:VMware1! https://localhost:443/v1/images \
  --insecure \
  --request POST \
  -F "file[]=@/home/kim/VMware-VMvisor-Installer-9.0.0.0.24755229.x86_64.iso"  | jq