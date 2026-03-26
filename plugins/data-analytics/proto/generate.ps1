$ErrorActionPreference = "Stop"
$OutputEncoding = [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

protoc `
  --go_out=../pb --go_opt=paths=source_relative `
  --go-grpc_out=../pb --go-grpc_opt=paths=source_relative `
  plugin.proto

Write-Output "Protobuf code generated in plugins/data-analytics/pb"
