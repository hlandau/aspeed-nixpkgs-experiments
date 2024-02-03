let
  pkgs = import <nixpkgs> {
    crossSystem.system = "armv6l-linux";
    crossSystem.config = "armv6l-unknown-linux-musleabihf";
    overlays = [ (self: super: {
      openssl = super.openssl.override {
        #enableKTLS = false;
      };
      openssh = super.openssh.override {
        withKerberos = false;
        withLdns = false;
        withPAM = false;
        linkOpenssl = false;
      };
      busybox = super.busybox.override {
        enableMinimal = false;
        enableStatic = false;
        #extraConfig = ''
        #  CONFIG_STATIC n
        #  CONFIG_STATIC_LIBGCC n
        #  CONFIG_BUILD_LIBBUSYBOX y
        #'';
      };
    })];
  };
  lib = pkgs.lib;
  stdenv = pkgs.stdenv;

  busybox = pkgs.busybox.override {
    enableMinimal = false; 
    enableStatic = false;
  };

  imageClosureInfo = pkgs.buildPackages.closureInfo { rootPaths = [
    pkgs.busybox
    pkgs.openssh
    busybox
  ]; };

  serviceImage = ./service/service;
  etc = ./etc;

in rec {
  deviceImage = stdenv.mkDerivation {
    name = "deviceImage";
    nativeBuildInputs = [
      pkgs.buildPackages.squashfsTools pkgs.buildPackages.cryptsetup pkgs.buildPackages.libllvm
      pkgs.buildPackages.rsync
    ];
    buildCommand = ''
      mkdir -p rootImage/nix/store
      xargs -I % cp -a --reflink=auto % -t ./rootImage/nix/store/ < ${imageClosureInfo}/store-paths

      chmod -R +rw rootImage/nix/store
      find rootImage/nix/store -name '*.mo' -exec rm -f '{}' ';'
      rm -rf rootImage/nix/store/*/share/i18n/locales
      rm -rf rootImage/nix/store/*/share/i18n/locale
      rm -rf rootImage/nix/store/*/share/i18n/charmaps
      rm -rf rootImage/nix/store/*/share/locale
      rm -rf rootImage/nix/store/*/share/man
      rm -rf rootImage/nix/store/*/lib/gconv
      rm -rf rootImage/nix/store/*-openssh-*/bin/{ssh,ssh-keyscan,ssh-add,ssh-agent,scp,sftp,ssh-copy-id}
      rm -rf rootImage/nix/store/*-openssl-*/etc
      rm -rf rootImage/nix/store/*-openssh-*/libexec/{sftp-server,ssh-*-helper,ssh-keysign}
      rm -rf rootImage/nix/store/*-openssh-*/etc
      rm -rf rootImage/nix/store/*-openssl-*/lib/{ossl-modules/legacy.so,engines-3/loader_attic.so,engines-3/afalg.so,engines-3/capi.so,engines-3/padlock.so}
      rm -rf rootImage/nix/store/*-busybox-*/default.script
      rm -rf rootImage/nix/store/*-ncurses-*/{bin,lib/libform*,lib/libmenu*,lib/libpanel*}

      find rootImage/nix/store -name '*.a' -exec rm '{}' ';'
      find rootImage/nix/store -name '*.la' -exec rm '{}' ';'
      find rootImage/nix/store -name '*.o' -exec rm '{}' ';'
      find rootImage/nix/store/*-gcc-* -name '*.py' -exec rm '{}' ';'
      find rootImage/nix/store/*-ncurses-*/share/terminfo | (
        while read n; do
          nx="$(basename "$n")"
          case "$nx" in
            xterm|xterm-color|vt100|screen|screen-16color|screen-256color|tmux|tmux-256color|linux|putty|putty-256color|rxvt|rxvt-256color)
              echo preserve "$nx"
              ;;
            *)
              rm -f "$n" &>/dev/null || true
              ;;
          esac
        done
      )
      find rootImage/nix/store -type d -exec rmdir '{}' ';' &>/dev/null || true

      mkdir -p rootImage/{bin,proc,sys,dev,tmp,etc/ssh,run,var/empty,mnt/cfgstore}
      cp -a "${serviceImage}" rootImage/bin/init
      llvm-strip -s rootImage/bin/init

      ln -s bin rootImage/sbin
      ln -s "${busybox}/bin/busybox" rootImage/bin/busybox
      for x in rootImage/nix/store/*busybox*/*bin/*; do
        ln -s busybox rootImage/bin/"$(basename "$x")" || true
      done

      ln -s "${pkgs.openssh}/bin/sshd" rootImage/bin/sshd
      ln -s "${pkgs.openssh}/bin/ssh-keygen" rootImage/bin/ssh-keygen
      rsync -a "${etc}/" rootImage/etc/

      mkdir -p "$out"
      ln -s "${deviceKernel.configfile}" "$out/kconfig"
      ln -s "${deviceKernel}" "$out/kernel"
      ln -s "${deviceKernel.dev}" "$out/kernel-dev"

      (cd rootImage; find . -type d -printf "%p m 0755 0 0\n";) | cut -d/ -f2- > sctl
      (cd rootImage; find . -type f -printf "%p m 0%m 0 0\n";) | cut -d/ -f2- >> sctl
      (cd rootImage; find . -type l -printf "%p m 0%m 0 0\n";) | cut -d/ -f2- >> sctl

      echo SCTL:
      cat sctl
      echo END SCTL

      mksquashfs rootImage/ "$out/root.squashfs" \
        -pf sctl -comp xz
      veritysetup format "$out/root.squashfs" "$out/root.verity"

      truncate -s 16M "$out/bmc-spi-half.img"
      dd if="$out/root.squashfs" conv=notrunc of="$out/bmc-spi-half.img"
      cat "$out/bmc-spi-half.img" "$out/bmc-spi-half.img" > "$out/bmc-spi.img"
    '';
  };

  deviceKernel = pkgs.linux_6_1.override {
    autoModules = false;
    preferBuiltin = true;
    defconfig = "aspeed_g5_defconfig";
    enableCommonConfig = false;

    # defconfig
    # extraConfig
    structuredExtraConfig = with pkgs.lib.kernel; {
      MODULES = yes;
      CC_OPTIMIZE_FOR_SIZE = yes;
      IKCONFIG = no;
      PWM = yes;
      VETH = yes;
      SENSORS_OCC_P8_I2C = yes;
      STRICT_KERNEL_RWX = yes;
      SLAB = yes;
      SLAB_FREELIST_RANDOM = yes;
      UNWINDER_FRAME_POINTER = yes;
      DEBUG_PINCTRL = yes;
      USB_CONFIGFS_ACM = yes;
      USB_CONFIGFS_ECM = yes;
      USB_CONFIGFS_F_FS = yes;
      KERNEL_LZMA = yes;
      #SQUASHFS_4K_DEVBLK_SIZE = yes;
      #MTD_CMDLINE_PARTS = yes;
    };
    # kernelPatches
    # preferBuiltin
    # autoModules
  };

#  opensshPackages = dontRecurseIntoAttrs (callPackage ../tools/networking/openssh {});
#  openssh = opensshPackages.openssh.override {
#    etcDir = "/etc/ssh";
#  };

}
