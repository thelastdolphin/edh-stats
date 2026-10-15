Placeholder that exists only so that packaging/data is a directory in the build
context. The Dockerfile copies it to /app/data with the runtime uid, which is how
the image gets a writable data directory without a RUN step.
