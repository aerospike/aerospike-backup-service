Name:           %{pkg_name}
Version:        %{pkg_version}
BuildArch:      %{build_arch}
Source0:        %{name}-%{version}.tar.gz
Release:        1
Summary:        Aerospike Backup Service
License:        Apache 2.0
Requires:       libuv


%description
Aerospike Backup Service

%prep
%setup -q -n "%{name}-%{version}"

%build
make build-submodules
make build

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}%{_bindir}
mkdir -p %{buildroot}%{_libdir}
mkdir -p %{buildroot}%{_mandir}/man1
mkdir -p %{buildroot}/%{_sharedstatedir}/%{name}
install -Dpm755 target/%{name} %{buildroot}%{_bindir}
install -Dpm755 lib/* %{buildroot}%{_libdir}
install -Dpm644 packages/man/%{name}.1 %{buildroot}%{_mandir}/man1/%{name}.1
install -Dpm644 packages/debian/%{name}.service %{buildroot}/lib/systemd/system/%{name}.service
install -Dpm644 packages/config/%{name}.yml %{buildroot}%{_sysconfdir}/%{name}/%{name}.yml

%files
%license LICENSE*
%doc *.md
%defattr(-,root,root)
%{_bindir}/%{name}
%{_libdir}/libasbackup.so
%{_libdir}/libasrestore.so
/lib/systemd/system/%{name}.service
%{_sysconfdir}/%{name}/%{name}.yml
%{_mandir}/man1/%{name}.1*
%dir %{_sharedstatedir}/%{name}

%post
/sbin/ldconfig
systemctl daemon-reload
systemctl enable %{name}
systemctl start %{name}

%preun
systemctl stop %{name}
systemctl disable %{name}

%postun
if [ $1 -eq 0 ] ; then
  rmdir %{_sysconfdir}/%{name}
fi

%clean
rm -rf %{buildroot}

%changelog
