# Distros

The Dockerfiles in this directory were adapted from the k0s project:
https://github.com/k0sproject/k0s/blob/2b7adcd574b0f20ccd1a2da515cf8c81d57b2241/inttest/bootloose-alpine/Dockerfile

These Dockerfiles are used to create various distribution images for testing and development purposes.

## Adding / updating distros

1. Create or modify the Dockerfile in the `dockerfiles` directory. For new distros, name it `<distro-name>.Dockerfile`.
1. Test building the image locally:
    ```bash
    make build-<distro-name>
    ```
1. Test an installation with the image locally.
1. Once merged to main, the CI will automatically (re)build and push the image to DockerHub.
