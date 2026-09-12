package plugins

import "testing"

func TestHasPermission_DefaultDeny(t *testing.T) {
	p := Plugin{Manifest: Manifest{Name: "legacy-plugin"}}
	for _, permission := range []string{PermissionRead, PermissionWrite, PermissionAnnotate} {
		if HasPermission(p, permission) {
			t.Fatalf("expected omitted permissions to deny %q", permission)
		}
	}
}

func TestHasPermission_ExplicitCapabilities(t *testing.T) {
	readOnly := Plugin{Manifest: Manifest{Permissions: []string{PermissionRead}}}
	if !HasPermission(readOnly, PermissionRead) {
		t.Fatal("expected explicit read capability")
	}
	if HasPermission(readOnly, PermissionWrite) {
		t.Fatal("read capability must not imply write")
	}
	if HasPermission(readOnly, PermissionAnnotate) {
		t.Fatal("read capability must not imply annotate")
	}

	writer := Plugin{Manifest: Manifest{Permissions: []string{PermissionWrite}}}
	if !HasPermission(writer, PermissionWrite) {
		t.Fatal("expected explicit write capability")
	}
	if !HasPermission(writer, PermissionAnnotate) {
		t.Fatal("write capability should include the narrower annotate capability")
	}
	if HasPermission(writer, PermissionRead) {
		t.Fatal("write capability must not implicitly grant read")
	}
}

func TestHasPermission_NormalizesManifestValues(t *testing.T) {
	p := Plugin{Manifest: Manifest{Permissions: []string{" READ ", "WRITE"}}}
	if !HasPermission(p, PermissionRead) || !HasPermission(p, PermissionWrite) {
		t.Fatalf("expected permission values to be normalized: %#v", p.Manifest.Permissions)
	}
}
