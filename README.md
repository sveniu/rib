`rib`: Root Image Build tool
============================
`rib` is a simple tool that makes it easy to build Linux root and initrd images
for network or container booting. Put your build scripts in a folder, and `rib`
will execute them in order, with flags that activate wrappers like fakeroot,
[fakechroot][1] and chroot. `rib` always runs as a normal user.

For example, you can build a fully functional [Debian image][2] that uses
squashfs with a tmpfs overlay, fitting everything conveniently inside a 64 MiB
initrd image.

It is highly recommended to run `rib` under a dedicated user account. While the
example build scripts are careful to verify that they modify the correct root
filesystem, it is very easy to shoot yourself in the foot.

`rib` is inspired by tools like [debirf][3] and [FAI][4].

[1]: https://github.com/dex4er/fakechroot/
[2]: https://github.com/sveniu/rib-examples/debian-jessie-small/
[3]: http://cmrg.fifthhorseman.net/wiki/debirf
[4]: http://fai-project.org/


Install
-------
```sh
go get github.com/sveniu/rib
```

Run
---
### Synopsis
```sh
rib init myimg
cd myimg
printf '#!/bin/sh\necho Test\n' > build.d/10--test
chmod +x build.d/10--test
rib build -v
```

Run `rib` to display the command usage. Main commands:

* `rib init`: Initialize a directory.

* `rib build`: Execute build scripts.

* `rib shell`: Execute an interactive shell inside a chroot, in the root
filesystem. This sets up a small `bashrc` for colored `ls` output and a simple
`PS1` prompt, and executes `/bin/bash`.

* `rib shell command...`: Execute the given command arguments interactively in
a chroot, in the root filesystem.

* `rib clean`: Delete contents of `dist/`, `rootfs/` and `tmp/`; recreate the
`fakeroot.save` file.

Once the kernel and initrd images are ready, test them with qemu:
```sh
qemu -nographic -m 512M -append "console=ttyS0" \
  -kernel dist/vmlinuz-XXX -initrd dist/initrd.cpio.gz
```


Build scripts
-------------

After initializing the rib directory with `rib init <dir>`, you can put any
kind of executable inside the `build.d` directory. These will be executed in
turn when you run `rib build`. The script filenames must include a sequence
number and flags, on the form `NN-FF-prog`; for example `25-RF-debootstrap`. To
run without flags, use `45--cleanup`.

Follow the build process with `rib build -v`, or inspect the build log written
to `log/build.log`.

The convention is to make build scripts put complete images or other finished
files into the directory pointed to by `RIB_DIR_DIST`; see below.


### Sequence numbers
The sequence number dictates script execution order. Any number of digits is
allowed. Use `rib build -s N` to only execute scripts with sequence number
equal to or higher than `N`.


### Execution flags
The following flags affect how the build script is executed:
* `I`: Interactive. Use this when a script requires user input. The script's
stdin, stdout and stderr will be available in the current terminal.
* `R`: Wrap in `fakeroot`. This will automatically use the `fakeroot.save` file
found in the main directory.
* `F`: Wrap in `fakechroot`.
* `C`: Run in `chroot`. The script is copied into the root filesystem, and
then executed through `fakechroot` + `fakeroot` + `chroot`; the `R` and `F`
flags are set implicitly.
* `E`: Ignore exit code. If the script fails, it will not stop the build
process.
* `S`: Skip this script. Useful while developing the build procedure.


### Runtime Environment
When build scripts execute, they have several environment variables available
for use. These vary depending on which script flags are used.


#### Scripts executing inside chroot
Default environment for scripts executing _with_ the `C` flag.

* `RIB_EXEC_ENV=1`, for easy verification of the execution environment.

* `PATH=/usr/sbin:/usr/bin:/sbin:/bin`.

* `VTEMP=/.volatile.XXXX`. This directory is removed after executing each
script, and is convenient to use for any sort of volatile storage.


#### Scripts executing outside chroot
Default environment for scripts executing _without_ the `C` flag:

* `RIB_EXEC_ENV=1`, for easy verification of the execution environment.

* `PATH=<rib_dir>/bin:/usr/sbin:/usr/bin:/sbin:/bin`.

* `VTEMP=<rib_dir>/tmp/.volatile.XXXX`. This directory is removed after
executing each script, and is convenient to use for any sort of volatile
storage.

* `RIB_DIR_BIN=<rib_dir>/bin`, the local bin directory. This is also
available as the first element of `PATH`.

* `RIB_DIR_BUILDD=<rib_dir>/build.d`, the build script directory.

* `RIB_DIR_DIST=<rib_dir>/dist`, the distribution directory intended to hold
output images, kernels, etc. NOTE: All content is erased when running `rib
clean --all`.

* `RIB_DIR_FILES=<rib_dir>/files`, holding auxiliary files, such as init
scripts, DHCP client hook scripts, etc.

* `RIB_DIR_LOG=<rib_dir>/log`, which usually only holds `build.log`. Put any
sort of log file here.

* `RIB_DIR_ROOTFS=<rib_dir>/rootfs`, holding the root filesystem. NOTE: All
content is erased when running `rib clean`.

* `RIB_DIR_TEMP=<rib_dir>/tmp`, where you can put "persistent" temporary
data that will not be automatically removed. NOTE: All content is erased when
running `rib clean`.


### Modifying Environment Variables
A build script can set environment variables that are made available to later
scripts. This is done by writing data to file descriptor 3, on the form
`<command> \x1f <key> \x1f <value> \x00`. The separator is the ASCII [unit
separator][5], represented in octal, decimal and hex by: `037`, `0x1F`, `31`.

These commands are available:
* `setenv`, `key=ENV_VAR_NAME`, `value=ENV_VAR_VALUE`: Set the specified
environment variable to the given value.
* `unsetenv`, `key=ENV_VAR_NAME`: Unset the specified environment variable.
Note that an empty `value` field must be included.

[5]: https://www.lammertbies.nl/comm/info/ascii-characters.html#unit


#### Examples

```sh
printf >&3 "%s\037%s\037%s\0" setenv "ENV_TEST" "foo"
printf >&3 "%s\037%s\037\0" unsetenv "ENV_TEST"
```

```python
os.write(3, '%s\037%s\037%s\0' % ('setenv', 'ENV_TEST', 'foo'))
os.write(3, '%s\037%s\037\0' % ('unsetenv', 'ENV_TEST'))
```


Best Practices
--------------
Tips for designing the build process:
* The `RIB_EXEC_ENV=1` environment variable is always set when `rib` is
executing build scripts, regardless of chroot status. It is a good idea to
verify its presence as a sanity check, especially for potentially dangerous
operations, such as sweeping deletions during clean-up.

* Early in the process, touch a file like `._RIB_ROOTFS_` inside the root
filesystem, and verify its presence in later scripts. This can help avoid risks
associated with shell quoting or a missing `C` chroot execution flag.

* For the same reason, consider running `rib` as a dedicated user.

* Always be mindful of implicit trust in binary packages and files. In other
words, it is essential to be aware of how the initial bootstrap process
verifies digital signatures. Never skip package or metadata verification simply
because it's more convenient to do so.

* On a Debian host system, `debootstrap` relies on the default-installed
`debian-archive-keyring` keyring. You can choose to rely on this implicit
trust, or make it more explicit by prompting the user – through a script with
the `I` interactive flag set – to verify signatures, checksums, key
fingerprints, etc.

* Make shell scripts exit on errors (`set -e`) and unexpanded variables
(`set -u`) to catch errors early.

* Use an early script to determine if essential commands like `debootstrap` and
`mksquashfs` are available on the host system. It's no fun to start the build
process and have it abort minutes later due to a missing command.

* A file created by a `C` chroot script can be copied out by putting the file
in a suitable location inside the chroot, and optionally pointing to it with an
environment variable set via file descriptor 3 (see above). A follow-up script
can then copy or move the file to its final location. Primitive, but effective.

* Use an early script to set environment variables that can be used for
configuring the build process. This makes build scripts more reusable. One
example is to export `DEBIAN_RELEASE=jessie` and then using it – with fallback
to a sensible default – when executing `debootstrap`.

* Use the `VTEMP` volatile directory to store temporary data instead of relying
on things like `mktemp(1)`. Not only is it convenient, but its location near
the rest of the `rib` data decreases the likelyhood of potentially expensive
cross-filesystem "copy and remove source" file move operations.

* Find a good balance between build-time and runtime configuration: A simple
root image can be further handled by a configuration management system once it
boots, so it's worth giving careful thought to how much the build-time
configuration should cover.


Motivation
----------
`rib` facilitates building root images from scratch, making the entire process
easy to repeat and automate. While any type of image can be created, the
primary motivation is to build generic images that can be booted over the
network. A network-booted system gives you several benefits:

* No need for a disk-based boot loader; no more GRUB vs LVM/LUKS/MDRAID
complexity.

* Optional persistent storage via LUKS+MDRAID+LVM. Fetch LUKS keys over the
network or from pstore to prevent casual snooping.

* Disk corruption doesn't affect system availability, so testing and
troubleshooting can happen immediately.

* A system upgrade can be as simple as a reboot.

* Best of all, a volatile root filesystem encourages users to carefully
consider the separation of configuration and volatile vs persistent data.

Despite the bold vision, `rib` is little more than a beefed-up alternative to
`run-parts(8)`. The original design plans involved being be able to bootstrap
both deb- and rpm-based systems and build a simple init that would mount the
squashfs image for further booting. This is akin to what [debirf][6] has been
doing for years – `rib` would modernize the approach a bit, using squashfs
together with tmpfs or zram, etc.

Over several iterations, it became increasingly clear that incorporating
specific image-building intelligence into the tool itself increases complexity
and reduces flexibility. By separating the build infrastructure (directory
management, dealing with fakeroot, fakechroot and chroot) from the scripts that
actually do the heavy lifting, the result is reduced complexity in `rib` itself
and greatly increased flexibility in the build scripts. The [build scripts
examples][7] are relatively simple shell scripts that produce useful images.

[6]: http://cmrg.fifthhorseman.net/wiki/debirf
[7]: https://github.com/sveniu/rib-examples/


Packaging
---------
Some considerations for packaging `rib` for distribution:

* While the `rib` binary itself is self-contained, it makes explicit calls to
the `fakeroot`, `fakechroot` and `chroot` commands – good candidates for
package dependencies. It may also make sense to use package metadata or
documentation to recommend some commonly used tools like `debootstrap`,
`squashfs-tools`, etc.

* It is recommended to run `rib` under a dedicated user account, to prevent
accidents with poor shell quoting or build scripts run with the wrong flags.
Communicating this to the user is a good idea. Unfortunately, using
setuid/setgid on the `rib` binary won't affect other executables such as build
scripts or `rib shell` execution.


License
-------
See `COPYING`.
