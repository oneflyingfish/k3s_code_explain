编译此程序请先查阅 pkg/data/readme.md并生成data.go，否则会报错

最后实质性执行的是`/var/lib/rancher/k3s/data/$HASH/bin/`下的程序：`k3s-server`| `k3s-agent` | `kubectl` | `crictl`| `ctr`|  `check-config`

此路径下文件大致为以下几个组件构成的软链接组成:

*busybox等程序通过命名为不同名称可表现出该名称对应的Linux命令的功能，详见手册*

| 类别 | 形式 | 源 | 文件名 |
| :-: | :-: |  :-: |  :- | 
| 软链接依赖 | 磁盘文件 | - | busybox, coreutils, pigz, k3s（非cmd/k3s编译得到的k3s二进制）, cni |
| 系统组件 | Linux软链接 | busybox | addgroup, adduser, ar, arch, arp, arping, ash, awk, bc, bunzip2, bzcat, chattr, chrt, chvt, clear, cmp, cpio, crond, crontab, dc, deallocvt, delgroup, deluser, devmem, diff, dnsd, dnsdomainname, dos2unix, dumpkmap, egrep, eject, ether-wake, fallocate, fbset, fdflush, fdformat, fgrep, free, freeramdisk, fsck, fsfreeze, fuser, getopt, getty, grep, gunzip, gzip, halt, hdparm, hexedit, hostname, hwclock, i2cdetect, i2cdump, i2cget, i2cset, i2ctransfer, ifconfig, ifdown, ifup, inetd, init, insmod, ipaddr, ipcrm, ipcs, iplink, ipneigh, iproute, iprule, iptunnel, killall, killall5, klogd, last, less, linux32, linux64, linuxrc, loadfont, loadkmap, logger, login, lsattr, lsmod, lsof, lspci, lsscsi, lsusb, lzcat, lzma, lzopcat, makedevs, mdev, mesg, microcom, mim, mkdosfs, mke2fs, mkpasswd, more, mountpoint, mt, nameif, netstat, nologin, nslookup, nuke, openvt, partprobe, passwd, patch, pidof, ping, pipe_progress, pivot_root, poweroff, ps, rdate, reboot, reset, resize, resume, rmmod, route, run-init, runlevel, run-parts, sed, setarch, setconsole, setfattr, setkeycodes, setlogcons, setpriv, setserial, sh, sha3sum, start-stop-daemon, strings, su, sulogin, svc, svok, switch_root, sysctl, syslogd, tar, telnet, tftp, time, top, traceroute, ts, ubirename, udhcpc, uevent, umount, unix2dos, unlzma, unlzop, unxz, unzip, usleep, uudecode, uuencode, vconfig, vi, vlock, w, watch, watchdog, wget, which, xxd, xz, xzcat, zcat |
| 系统组件 | Linux软链接 | coreutils | b2sum, base32, base64, basename, basenc, cat, chcon, chgrp, chmod, chown, chroot, cksum, comm, cp, csplit, cut, date, dd, df, dir, dircolors, dirname, du, echo, env, expand, expr, factor, false, fmt, fold, groups, head, hostid, id, install, join, kill, link, ln, logname, ls, md5sum, mkdir, mkfifo, mknod, mktemp, mv, nice, nl, nohup, nproc, numfmt, od, paste, pathchk, pinky, pr, printenv, printf, ptx, pwd, readlink, realpath, rm, rmdir, runcon, seq, sha1sum, sha224sum, sha256sum, sha384sum, sha512sum, shred, shuf, sleep, sort, split, stat, stty, sum, sync, tac, tail, tee, test, timeout, touch, tr, true, truncate, tsort, tty, uname, unexpand, uniq, unlink, uptime, users, vdir, wc, who, whoami, yes |
| 系统组件 | Linux软链接 | pigz | unpigz |
| 其它 | 磁盘文件 | - | aux, blkid,  ethtool, find, fuse-overlayfs, ip, ipset, losetup, nsenter, slirp4netns, swanctl, charon, swanctl |
| k3s组件 | Linux软链接 | k3s（非cmd/k3s编译得到的k3s二进制，疑似cmd/containerd 待验证） | crictl, ctr, k3s-agent, k3s-server, kubectl, k3s-certificate, k3s-etcd-snapshot, k3s-secrets-encrypt |
| k3s网络插件 | Linux软链接 | cni | bridge, flannel, host-local, loopback, portmap | 
| k3s-containerd运行时 | 磁盘文件 | - | conntrack, containerd, containerd-shim-runc-v2, check-config, runc |