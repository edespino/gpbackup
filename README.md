# Greenplum Backup

gpbackup and gprestore are Go utilities for performing backups and restores of a Greenplum Database. They are still currently in active development.

## Pre-Requisites

gpbackup requires Go version 1.8 or higher.
Follow the directions [here](https://golang.org/doc/) to get the language set up.

## Downloading

```bash
go get github.com/greenplum-db/gpbackup/...
```

This will place the code in `$GOPATH/github.com/greenplum-db/gpbackup`.

## Building and installing binaries

cd into the gpbackup directory and run

```bash
make depend
make build
```

This will put the gpbackup and gprestore binaries in `$HOME/go/bin`

`make build_linux` and `make build_mac` are for cross compiling between macOS and Linux

`make install_helper` will scp the gpbackup_helper binary (used with -single-data-file flag) to all hosts

## Running the utilities

The basic command for gpbackup is
```bash
gpbackup --dbname <your_db_name>
```

The basic command for gprestore is
```bash
gprestore --timestamp <YYYYMMDDHHMMSS>
```

Run `--help` with either command for a complete list of options.

## Validation and code quality

To run all tests except end-to-end (unit, integration, and linters), use
```bash
make test
```
To run only unit tests, use
```bash
make unit
```
To run only integration tests (which require a running GPDB instance), use
```bash
make integration
```

To run end to end tests, use
```bash
make end_to_end
```

**We provide the following targets to help developers ensure their code fits Go standard formatting guidelines.**

To run a linting tool that checks for basic coding errors, use
```bash
make lint
```
This target runs [gometalinter](https://github.com/alecthomas/gometalinter).

Note: The lint target will fail if code is not formatted properly.


To automatically format your code and add/remove imports, use
```bash
make format
```
This target runs [goimports](https://godoc.org/golang.org/x/tools/cmd/goimports) and [gofmt](https://golang.org/cmd/gofmt/).
We will only accept code that has been formatted using this target or an equivalent `gofmt` call.

## Cleaning up

To remove the compiled binaries and other generated files, run
```bash
make clean
```

# More Information

The GitHub wiki for this project has several articles providing a more in-depth explanation of certain aspects of gpbackup and gprestore.

# How to Contribute

We accept contributions via [Github Pull requests](https://help.github.com/articles/using-pull-requests) only.

Follow the steps below to contribute to gpbackup:
1. Fork the project’s repository.
1. Run `go get github.com/greenplum-db/gpbackup/...` and add your fork as a remote.
1. Run `make depend` to install required dependencies
1. Create your own feature branch (e.g. `git checkout -b gpbackup_branch`) and make changes on this branch.
    * Follow the previous sections on this page to setup and build in your environment.
    * Add new tests to cover your code. We use [Ginkgo](http://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) for testing.
1. Run `make format`, `make test`, and `make end_to_end` in your feature branch and ensure they are successful.
1. Push your local branch to the fork (e.g. `git push <your_fork> gpbackup_branch`) and [submit a pull request](https://help.github.com/articles/creating-a-pull-request).

Your contribution will be analyzed for product fit and engineering quality prior to merging.
Note: All contributions must be sent using GitHub Pull Requests.

**Your pull request is much more likely to be accepted if it is small and focused with a clear message that conveys the intent of your change.**

Overall we follow GPDB's comprehensive contribution policy. Please refer to it [here](https://github.com/greenplum-db/gpdb#contributing) for details.

# Troubleshooting

On macOS, if you see errors in many integration tests, such as:

```
SECURITY LABEL FOR dummy ON TYPE public.testtype IS 'unclassified';
      Expected
          <pgx.PgError>: {
              Severity: "ERROR",
              Code: "22023",
              Message: "security label provider \"dummy\" is not loaded",
```

then you need to load a "dummy" security label, by using the gpdb tool in `gpdb/contrib/dummy_seclabel`. A utility script for this is:

```bash
pushd ~/workspace/gpdb/contrib/dummy_seclabel
    make install
    gpconfig -c shared_preload_libraries -v dummy_seclabel
    gpstop -ra
    gpconfig -s shared_preload_libraries | grep dummy_seclabel
popd

```

On macOS, if you see errors in gpdb master like:

```
configure: error: zstd library not found.
```

one way to get around this is to change configuration to compile GPDB with fewer features: 

```bash
$ cd gpdb
$ ./configure --without-zstd --disable-orca --with-perl --with-python --with-libxml --with-gssapi --disable-gpfdist --with-openssl --prefix=/usr/local/gpdb && make -j8
```

On macOS, if you see errors like:

```bash
checking for apr-1-config
cannot find apr-1-config
```

one way to get around this is to add some special paths to shell, adding these to ~/.bashrc :

```bash
export PATH="/usr/local/opt/apr/bin:$PATH"
export PATH="/usr/local/opt/openssl/bin:$PATH"
```

(The problem is that the newest Apple Command Line Tools includes some of these binaries, so `brew` no longer will link such binaries.)
