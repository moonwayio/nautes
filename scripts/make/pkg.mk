ifndef _PKGTYPE_DEFINED
_PKGTYPE_DEFINED := 1

.PHONY: _pkgtype
_pkgtype:
	@if command -v brew >/dev/null; then \
		echo "brew"; \
	elif command -v apk >/dev/null; then \
		echo "apk"; \
	elif command -v apt-get >/dev/null; then \
		echo "apt-get"; \
	elif command -v dnf >/dev/null; then \
		echo "dnf"; \
	elif command -v yum >/dev/null; then \
		echo "yum"; \
	elif command -v pacman >/dev/null; then \
		echo "pacman"; \
	else \
		echo "unknown"; \
	fi
endif
