package permissions

import "testing"

func TestAtLeast(t *testing.T) {
	if !AtLeast(RoleAdmin, RoleEngineer) {
		t.Fatal("admin should satisfy at-least engineer")
	}
	if AtLeast(RoleViewer, RoleAnalyst) {
		t.Fatal("viewer should not satisfy at-least analyst")
	}
}

func TestValid(t *testing.T) {
	if !Valid(RoleAdmin) {
		t.Fatal("admin should be a valid role")
	}
	if Valid(Role("superuser")) {
		t.Fatal("unknown role should not be valid")
	}
}
