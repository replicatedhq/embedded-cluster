#!/usr/bin/env expect

set timeout 600 # 10 minutes

local dr_aws_s3_endpoint="$1"
local dr_aws_s3_region="$2"
local dr_aws_s3_bucket="$3"
local dr_aws_s3_prefix="$4"
local dr_aws_access_key_id="$5"
local dr_aws_secret_access_key="$6"

spawn embedded-cluster restore

expect "Enter information to configure access to your backup storage location."

expect "S3 endpoint:"
send "$dr_aws_s3_endpoint\n"

expect "Region:"
send "$dr_aws_s3_region\n"

expect "Bucket:"
send "$dr_aws_s3_bucket\n"

expect "Prefix (press Enter to skip):"
send "$dr_aws_s3_prefix\n"

expect "Access key ID:"
send "$dr_aws_access_key_id\n"

expect "Secret access key:"
send "$dr_aws_secret_access_key\n"

expect "Velero is ready!"
expect "Backup storage location configured!"
expect "Restore from backup"
send "Y\n"

expect "Infrastructure restored!"
expect "Cluster state restored!"
expect "Application restored!"

expect "Visit the admin console if you need to add nodes to the cluster: http://10.0.0.2:30000"
expect "Type 'continue' when you are done adding nodes:"
send "continue\n"

expect "All nodes are ready!"
expect "Application restored!"

expect eof
