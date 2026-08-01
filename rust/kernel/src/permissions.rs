/// HermiNas' four RBAC roles (cahier des charges §7.1). Mirrors
/// kernel/permissions/permissions.go and herminas_kernel.permissions
/// (Python). Full RBAC middleware lands in M1.5/M7.1.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Role {
    Viewer,
    Analyst,
    Engineer,
    Admin,
}

impl Role {
    pub fn rank(self) -> u8 {
        match self {
            Role::Viewer => 0,
            Role::Analyst => 1,
            Role::Engineer => 2,
            Role::Admin => 3,
        }
    }

    pub fn at_least(self, min: Role) -> bool {
        self.rank() >= min.rank()
    }

    pub fn all() -> [Role; 4] {
        [Role::Viewer, Role::Analyst, Role::Engineer, Role::Admin]
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn at_least_respects_hierarchy() {
        assert!(Role::Admin.at_least(Role::Engineer));
        assert!(!Role::Viewer.at_least(Role::Analyst));
    }
}
