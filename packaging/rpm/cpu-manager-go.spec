# SPEC file per cpu-manager-go
# Build con: rpmbuild -ba cpu-manager-go.spec

Name:    cpu-manager-go
Version: 1.0.0
Release: 1%{?dist}
Summary: Dynamic CPU resource management tool using cgroups v2

License: GPLv3
URL:     https://github.com/fdefilippo/cpu-manager-go
Source0: %{name}-%{version}.tar.gz

## Disable debug packages.
%define debug_package %{nil}

## Disable build_id
%define _build_id_links none

%if 0%{?rhel} == 8
%define __brp_mangle_shebangs /usr/bin/true
%endif

# Dichiara che il package contiene una man page
%global _has_manpage 1

BuildRequires:  golang >= 1.21
BuildRequires:  systemd
BuildRequires:  groff-base
Requires:       systemd
Requires:       golang >= 1.21

# Dipendenze cgroups
Requires(post): systemd-units
Requires(preun): systemd-units
Requires(postun): systemd-units

%description
Enterprise-grade CPU resource management tool with cgroups v2 support.
Automatically limits CPU for non-system users based on configurable thresholds.

Features:
- Dynamic CPU limiting for non-system users
- Configurable activation/release thresholds
- Prometheus metrics export
- Systemd service integration
- Automatic configuration reload on changes
- Detailed process logging
- Complete man page documentation

%package doc
Summary: Documentation for %{name}
License: MIT
Requires: %{name} = %{version}-%{release}

%description doc
Documentation for %{name}, including man page and configuration examples.

%prep
%setup -q

%build
# Build del binario Go
export GO111MODULE=on
export GOPROXY=direct
go build -v -ldflags="-s -w -X 'main.version=%{version}-%{release}'" -o %{name}

# Prepara man page
mkdir -p %{_builddir}/%{name}-%{version}/man
cp docs/cpu-manager.8 %{_builddir}/%{name}-%{version}/man/
gzip -9 %{_builddir}/%{name}-%{version}/man/cpu-manager.8

%install
# Crea directory
mkdir -p %{buildroot}/%{_bindir}
mkdir -p %{buildroot}/%{_sysconfdir}
mkdir -p %{buildroot}/%{_unitdir}
mkdir -p %{buildroot}/%{_sharedstatedir}/cpu-manager
mkdir -p %{buildroot}/%{_localstatedir}/log
mkdir -p %{buildroot}/%{_mandir}/man8
mkdir -p %{buildroot}/%{_docdir}/%{name}-%{version}

# Installa binario
install -m 755 %{name} %{buildroot}/%{_bindir}/%{name}

# Installa file di configurazione
install -m 644 config/cpu-manager.conf.example %{buildroot}/%{_sysconfdir}/cpu-manager.conf

# Installa service systemd
install -m 644 packaging/systemd/cpu-manager.service %{buildroot}/%{_unitdir}/

# Installa man page
install -m 644 %{_builddir}/%{name}-%{version}/man/cpu-manager.8.gz %{buildroot}/%{_mandir}/man8/

# Installa documentazione aggiuntiva
install -m 644 README.md %{buildroot}/%{_docdir}/%{name}-%{version}/
install -m 644 LICENSE %{buildroot}/%{_docdir}/%{name}-%{version}/
install -m 644 config/cpu-manager.conf.example %{buildroot}/%{_docdir}/%{name}-%{version}/

# Crea directory per runtime files
install -d -m 755 %{buildroot}/%{_sharedstatedir}/cpu-manager

%pre
# Pre-install script
if [ $1 -eq 1 ]; then
    # Nuova installazione
    echo "Preparing for CPU Manager installation..."

    # Verifica cgroups v2
    if [ ! -f /sys/fs/cgroup/cgroup.controllers ]; then
        echo "WARNING: cgroups v2 not detected. Please enable with:"
        echo "  grubby --update-kernel=ALL --args='systemd.unified_cgroup_hierarchy=1'"
        echo "  reboot"
    fi
fi

%post
# Post-install script
%systemd_post cpu-manager.service

# Crea file di log
touch /var/log/cpu-manager.log
chmod 644 /var/log/cpu-manager.log

# Aggiorna database man page
%{_bindir}/mandb -q 2>/dev/null || true

# Abilita cgroup controllers se non già abilitati
if ! grep -q "+cpu" /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null; then
    echo "+cpu" >> /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null || true
fi
if ! grep -q "+cpuset" /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null; then
    echo "+cpuset" >> /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null || true
fi

echo "CPU Manager installed successfully!"
echo ""
echo "Configuration file: /etc/cpu-manager.conf"
echo "Log file: /var/log/cpu-manager.log"
echo "Service: systemctl start cpu-manager"
echo "Documentation: man cpu-manager"
echo ""
echo "Please review /etc/cpu-manager.conf before starting the service."

%preun
# Pre-uninstall script
%systemd_preun cpu-manager.service

%postun
# Post-uninstall script
%systemd_postun_with_restart cpu-manager.service

# Aggiorna database man page
%{_bindir}/mandb -q 2>/dev/null || true

# Rimuove directory runtime (solo se vuota)
rmdir /var/run/cpu-manager 2>/dev/null || true

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%config(noreplace) %{_sysconfdir}/cpu-manager.conf
%{_unitdir}/cpu-manager.service
%{_mandir}/man8/cpu-manager.8.gz
%dir %{_sharedstatedir}/cpu-manager

%files doc
%license LICENSE
%doc README.md
%doc %{_docdir}/%{name}-%{version}/*

%changelog
* Thu Jan 22 2026 CPU Manager <francesco@defilippo.org> - 1.0.0-1
- Initial RPM release with man page support
- Complete cgroups v2 CPU management
- Prometheus metrics support
- Dynamic configuration reload
- Systemd service integration
- Comprehensive man page documentation
