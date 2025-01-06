# S3 bucket object synchronisation service

This is background service that implements a couple of workflows to ensure that changes to folder on the local host are synchronised in near real-time to an area in an S3 bucket, and that files updated in an area of an S3 bucket are synchronised in near real-time to the local host, avoiding potential loops.

It is written in Golang.
