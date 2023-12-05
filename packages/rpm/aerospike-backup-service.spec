Name:           %{pkg_name}
Version:        %{pkg_version}
BuildArch:      %{build_arch}
Source0:        %{name}-%{version}.tar.gz
Release:        1%{?dist}
Summary:        Aerospike Backup Service
License:        Apache 2.0
Requires:       libuv


%description
Aerospike Backup Service

%prep
%setup -q -n "%{name}-%{version}"
make install-aws-sdk-cpp
make install-libuv
make install-jansson

%build
make build-submodules
make build

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}%{_bindir}
mkdir -p %{buildroot}%{_libdir}
mkdir -p %{buildroot}%{_mandir}/man1
install -Dm755 target/%{name} %{buildroot}%{_bindir}
install -Dm755 lib/* %{buildroot}%{_libdir}
install -pm644 packages/man/%{name}.1 %{buildroot}%{_mandir}/man1/%{name}.1

%files
%license LICENSE*
%doc *.md
%defattr(-,root,root)
%{_bindir}/%{name}
%{_libdir}/libasbackup.so
%{_libdir}/libasrestore.so
%{_mandir}/man1/%{name}.1*

%clean
rm -rf %{buildroot}

%post
/sbin/ldconfig

%changelog
