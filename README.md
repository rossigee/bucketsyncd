# Bucket synchronisation service

## Introduction

This application provides a bucket synchronisation service that provides a way to automatically download files to a local folder as they appear in a remote bucket, or to automatically upload files to a remote bucket or WebDAV server from a local folder as they are written to it.

### Features

*   **Push synchronisation**: Watches for local file system changes in specified locations and synchronises new or updated files to a specified bucket or WebDAV server.
*   **Pull synchronisation**: Can process S3 upload event notifications from a message queue, downloading new or updated files to the local machine.
*   **Multiple Storage Backends**: Supports both S3-compatible storage (MinIO, AWS S3) and WebDAV servers.
*   **Secure Protocols**: Supports both HTTP (`webdav://`) and HTTPS (`webdavs://`) WebDAV connections.
*   **Custom Filtering**: (TODO) Ability to process the file with a script before upload/download, which useful for removing or obfuscating sensitive data.

## Usage

To use the bucket synchronisation service, follow these steps:

1.  **Storage access**: Ensure that buckets exist in your cloud storage solution and that you have S3-compatible credentials with appropriate access, OR ensure you have WebDAV server access with valid credentials.
2.  **Message queue access**: If you are configuring a pull synchronisation, you will also need to ensure the service has access to the relevant virtualhost and queue.
3.  **Configure Service**: Configure the service by providing details about your storage solution. See [`example/config.yaml`](example/config.yaml).
4.  **Start Service**: Ensure the service is started and runs in the background. You can do this with a user-based `systemctl` configuration.

## Storage Backend Support

### S3-Compatible Storage
For S3-compatible storage (AWS S3, MinIO, etc.), use URLs in the format:
```
s3://endpoint.com/bucket-name/path
```

Configure S3 credentials in the `remotes` section of your config file.

### WebDAV Storage
For WebDAV servers, use URLs in the format:
```
webdav://username:password@server.com/path          # HTTP
webdavs://username:password@server.com/path         # HTTPS (secure)
```

WebDAV credentials are embedded directly in the URL. No separate remote configuration is needed.

**WebDAV Features:**
- Automatic directory creation on the remote server
- Support for both HTTP and HTTPS connections
- Username/password authentication
- Compatible with popular WebDAV servers (Apache, nginx, Nextcloud, etc.)

## Configuration example

Copy the [`example/bucketsyncd.service`] systemd unit file to your home directory as `~/.config/systemd/user/bucketsyncd.service`. Update it to reflect the locations of where your binary and configuration files are.

Then:

```sh
systemctl --user enable bucketsyncd
```

Check it's running as expected.

```sh
systemctl --user status bucketsyncd
```

## Contributing

To contribute to the bucket synchronisation service, follow these steps:

1.  **Fork Repository**: Fork the repository on GitHub.
2.  **Create Pull Request**: Create a pull request with your changes.
3.  **Review Changes**: Review changes and provide feedback to other contributors.

## License

The bucket synchronisation service is licensed under the MIT license, which allows for free use and modification of the software.
