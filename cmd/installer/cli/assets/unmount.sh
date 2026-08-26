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
awk -v d="$dir" '
    index($2, d "/") == 1 { mounts[++n] = $2 }
    END { for (i = n; i > 0; i--) print mounts[i] }
' /proc/self/mounts |
while IFS= read -r mount; do
    # A mount still pinned by an orphaned container process needs a lazy detach.
    umount "$mount" 2>/dev/null && continue
    umount -l "$mount" 2>/dev/null || true
done
