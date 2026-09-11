#!/bin/sh
# Detach every mount below a directory.
#
# reset runs this before removing the k0s data and run directories when
# `k0s reset` did not get far enough to do it itself.
# rm returns EBUSY on a mountpoint, and a torn-down node is left covered in
# them: kubelet leaves tmpfs and bind mounts under the pod volume tree, and
# containerd leaves sandbox shm and task rootfs overlays under the run dir.
#
# Usage: unmount.sh <directory>

set -u

dir="$1"

# Field 2 of the mount table is the mount point. Collect the ones strictly below
# $dir, then emit them in reverse mount order so that nested mounts and
# over-mounts are detached before the mounts they sit on.
#
# $dir itself is deliberately excluded: if it is a mount, it is one the user
# provided (a data dir on its own partition), not one we created. k0s leaves
# those alone for the same reason, and rm still clears the contents.
#
# The kernel escapes space and tab in mount points, so those are decoded before
# comparing — a data dir containing a space would otherwise match nothing. $dir
# is passed through the environment rather than -v, which would itself interpret
# backslash escapes in the value.
DIR="$dir" awk '
    function unescape(p) {
        gsub(/\\040/, " ", p)
        gsub(/\\011/, "\t", p)
        return p
    }
    BEGIN { d = ENVIRON["DIR"] }
    { mountpoint = unescape($2) }
    index(mountpoint, d "/") == 1 { mounts[++n] = mountpoint }
    END { for (i = n; i > 0; i--) print mounts[i] }
' /proc/self/mounts |
while IFS= read -r mount; do
    # A mount still pinned by an orphaned container process needs a lazy detach.
    umount "$mount" 2>/dev/null && continue
    umount -l "$mount" 2>/dev/null || true
done
