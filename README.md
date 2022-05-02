# Windows S3 synchronisation service

This is Windows service that implements a couple of workflows to ensure that changes to folder on the Windows host are synchronised in near real-time to an area in an S3 bucket, and that files updated in an area of an S3 bucket are synchronised in near real-time to the Windows host, avoiding potential loops.

It is written in Golang.
