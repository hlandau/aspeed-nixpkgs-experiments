package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func supervise(cmdFactory func() (*exec.Cmd, error)) {
	go func() {
		for {
			cmd, err := cmdFactory()
			if err != nil {
				fmt.Printf("cmd factory failed: %v\n", err)
				break
			}

			err = cmd.Start()
			if err != nil {
				fmt.Printf("failed to spawn cmd: %v\n", err)
				time.Sleep(5 * time.Second)
				continue
			}

			err = cmd.Wait()
			if err != nil {
				fmt.Printf("command exited unsuccessfully: %v\n", err)
			} else {
				fmt.Printf("command exited successfully\n")
			}
		}
	}()
}

func mustRun(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(fmt.Sprintf("command failed: %v %v: %v", name, args, err))
	}
}

func cmdInit() {
	signal.Ignore(syscall.SIGALRM, syscall.SIGCONT, syscall.SIGHUP, syscall.SIGINT, syscall.SIGPIPE, syscall.SIGTERM)

	_, err := syscall.Setsid()
	if err != nil {
		panic("setsid")
	}

	// mount -t sysfs -o noexec,nosuid,nodev sysfs /sys
	err = syscall.Mount("sysfs", "/sys", "sysfs", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, "")
	if err != nil {
		panic("cannot mount sysfs")
	}

	// mount -t tmpfs -o exec,nosuid,mode=0755,size=2M tmpfs /dev
	err = syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID, "mode=0755,size=2097152")
	if err != nil {
		panic(fmt.Sprintf("cannot mount /dev: %v", err))
	}

	// mknod -m 666 /dev/null c 1 3
	err = syscall.Mknod("/dev/null", syscall.S_IFCHR|0666, 0x1_03)
	if err != nil {
		panic(fmt.Sprintf("cannot make /dev/null: %v", err))
	}

	// mount -t proc -o noexec,nosuid,nodev proc /proc
	err = syscall.Mount("proc", "/proc", "proc", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, "")
	if err != nil {
		panic(fmt.Sprintf("cannot mount /proc: %v", err))
	}

	// mknod -m 666 /dev/ptmx c 5 2
	err = syscall.Mknod("/dev/ptmx", syscall.S_IFCHR|0666, 0x5_02)
	if err != nil {
		panic("ptmx")
	}

	err = syscall.Mkdir("/dev/pts", 0755)
	if err != nil {
		panic("mkdir.pts")
	}

	// mount -t devpts devpts -o noexec,nosuid,mode=0620,gid=5 /dev/pts
	err = syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOEXEC|syscall.MS_NOSUID, "mode=0620,gid=5")
	if err != nil {
		panic("devpts")
	}

	err = syscall.Mkdir("/dev/shm", 0777)
	if err != nil {
		panic("mkdir.shm")
	}

	// mount -t tmpfs -o nodev,nosuid,noexec shm /dev/shm
	err = syscall.Mount("shm", "/dev/shm", "tmpfs", syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
	if err != nil {
		panic("mount.shm")
	}

	// mount -t tmpfs -o nodev,nosuid,noexec run /run
	err = syscall.Mount("run", "/run", "tmpfs", syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
	if err != nil {
		panic("mount.run")
	}

	// mount -t tmpfs -o nodev,nosuid,noexec tmp /tmp
	err = syscall.Mount("tmp", "/tmp", "tmpfs", syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
	if err != nil {
		panic("mount.tmp")
	}

	// mdev
	mustRun("/bin/mdev", "-s")

	// ----------- end pre-init ----------

	// kernel.sysrq=0
	//
	// net.ipv4.tcp_syncookies=1
	// net.ipv4.tcp_synack_retries=5
	// net.ipv4.conf.all.send_redirects=0
	// net.ipv4.conf.default.send_redirects=0
	// net.ipv4.conf.all.accept_source_route=0
	// net.ipv4.conf.all.accept_redirects=0
	// net.ipv4.conf.all.secure_redirects=0
	// net.ipv4.conf.default.accept_source_route=0
	// net.ipv4.conf.default.accept_redirects=0
	// net.ipv4.conf.default.secure_redirects=0
	// net.ipv4.icmp_echo_ignore_broadcasts=1
	// net.ipv4.conf.all.rp_filter=1
	// net.ipv4.conf.default.rp_filter=1
	//
	// net.ipv6.conf.default.router_solicitations=0
	//
	// kernel.exec-shield=1 or 2
	// kernel.randomize_va_space=...
	// kernel.panic=1
	//
	// fs.protected_hardlinks=1
	// fs.protected_symlinks=1
	// kernel.dmesg_restrict=0

	// mount -t tmpfs -o nodev,nosuid,noexec cfgstore /mnt/cfgstore
	err = syscall.Mount("cfgstore", "/mnt/cfgstore", "tmpfs", syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
	if err != nil {
		panic("mount.cfgstore")
	}

	hostname := "bmc1"
	fqdn := hostname + ".lhh.devever.net"

	err = syscall.Sethostname([]byte(hostname))
	if err != nil {
		panic("hostname")
	}

	// networking: fqdn
	err = ioutil.WriteFile("/run/hosts", []byte("127.0.0.1 "+fqdn+" "+hostname+" localhost\n"), 0644)
	if err != nil {
		panic("hosts")
	}

	// networking: resolv.conf
	err = ioutil.WriteFile("/run/resolv.conf", []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n"), 0644)
	if err != nil {
		panic("resolv.conf")
	}

	mustRun("/bin/ip", "link", "set", "eth0", "up")
	mustRun("/bin/ip", "addr", "add", "192.168.1.99/24", "dev", "eth0")
	mustRun("/bin/ip", "route", "add", "default", "via", "192.168.1.1", "dev", "eth0")

	// ----------- end networking early init ----------
	mustRun("/bin/ssh-keygen", "-t", "ed25519", "-N", "", "-f", "/run/ssh_host_ed25519_key")

	// ----------- end ssh init ----------

	for _, tty := range []string{"/dev/ttyS4"} {
		ttyx := tty
		supervise(func() (*exec.Cmd, error) {
			shCmd := exec.Command("/bin/getty", "-wt", "30", "115200", ttyx)
			shCmd.Stdin = os.Stdin
			shCmd.Stdout = os.Stdout
			shCmd.Stderr = os.Stderr
			return shCmd, nil
		})
	}

	supervise(func() (*exec.Cmd, error) {
		cmd := exec.Command("/bin/ntpd", "-n")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd, nil
	})

	supervise(func() (*exec.Cmd, error) {
		cmd := exec.Command("/bin/sshd", "-D", "-f", "/etc/ssh/sshd_config")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd, nil
	})

	go func() {
		err := http.ListenAndServe(":80", nil)
		if err != nil {
			panic("http listen")
		}
	}()
}

func main() {
	fmt.Println("despair-init")
	cmdInit()
	fmt.Println("slumber start")
	for {
		time.Sleep(1 * time.Hour)
	}
}
