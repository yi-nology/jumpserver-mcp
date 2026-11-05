/*
 * rbac —— 角色与 RBAC 角色矩阵。
 */
namespace go jmsms

/**
 * jms_system_role_permissions 的一行。
 *
 * 这个工具有两种答法：不点名角色时答**系统角色清单**，点名时答**那个角色
 * 能做的操作路径**。一个工具一个契约类型（见 AGENTS.md 的契约纪律），所以
 * 行类型是两种答法的并集 —— 哪一半有值，看这次答的是哪一种，答复的 `note`
 * 里也写着是哪一种。
 */
struct SystemRolePermissionRow {
  /** 角色清单模式：角色的 uuid。 */
  1: optional string id,
  /** 角色清单模式：角色名。 */
  2: optional string name,
  /** 角色清单模式：显示名。 */
  3: optional string display_name,
  /** 角色清单模式：是不是内置角色。 */
  4: optional bool builtin,
  /** 角色清单模式：几个用户持有它。 */
  5: optional i32 users_amount,
  /** 角色清单模式：备注。 */
  6: optional string comment,
  /** 权限模式：一条 'app.action' 路径。 */
  7: optional string path,
}

/** 一个角色（jms_roles_list 的一行）。 */
struct RoleSummary {
  1: optional string id,
  2: optional string name,
  3: optional string display_name,
  /** 作用域：org / system。 */
  4: optional string scope,
  /** 内置角色不能删（只能复制成自定义的）。 */
  5: optional bool builtin,
  6: optional i32 users_amount,
}

/** 一个角色能做的一件事（jms_role_permissions 的一行）。 */
struct RolePermissionSummary {
  /** 权限路径，如 "assets.add"。 */
  1: required string path,
}

/**
 * RBAC 权限目录里的一条（jms_rbac_permissions 的一行）。
 *
 * 这是**权限本身的目录**（一条 = 一个 app.action 能力，如 add_account），
 * 与资产授权（jms_permissions_list 的一行 = 一条「谁能进哪些机器」的授权）
 * 不是一回事：目录说「系统里定义了哪些能力」，授权说「谁被允许干什么」。
 */
struct RBACPermission {
  /** 它的 id（数字，权限目录的主键）。 */
  1: optional string id,
  /** 人读的名字，如 "Can add Account"。 */
  2: optional string name,
  /** 能力码，如 add_account。 */
  3: optional string codename,
  /** 属于哪个应用（content_type.app_label），如 accounts。 */
  4: optional string app_label,
  /** 属于哪个模型（content_type.model），如 account。 */
  5: optional string model,
}

/**
 * 角色权限矩阵里的一列（参与矩阵的一个角色）。
 */
struct RBACMatrixRole {
  /** 角色 id。 */
  1: optional string id,
  /** 角色名。 */
  2: optional string name,
  /** 作用域：org / system。 */
  3: optional string scope,
  /** 是不是内置角色（内置的不能删）。 */
  4: optional bool builtin,
  /** 有多少用户持有它。 */
  5: optional i32 users_amount,
  /** 它持有多少条权限（矩阵里数出来的）。 */
  6: optional i32 permission_count,
}

/**
 * 角色权限矩阵里的一行（一条权限 + 哪些角色持有它）。
 */
struct RBACMatrixEntry {
  /** 属于哪个应用。 */
  1: optional string app_label,
  /** 属于哪个模型。 */
  2: optional string model,
  /** 能力码。 */
  3: optional string codename,
  /** 人读的名字。 */
  4: optional string name,
  /** 持有它的角色名（空 = 没有任何角色持有，是条死权限）。
   *  required 而不是 optional：空数组就是这工具的发现之一，omitempty 会把它整个键吃掉。 */
  5: required list<string> holders,
}

/**
 * 角色 × 权限的矩阵（jms_rbac_role_matrix 的答复）。
 */
struct RBACRoleMatrix {
  /** 参与矩阵的角色（列）。required：矩阵答复要能 diff，空列表也得占位（omitempty 会吃键）。 */
  1: required list<RBACMatrixRole> roles,
  /** 权限行（受 search/limit 约束）。required：同上，空矩阵也是答复。 */
  2: required list<RBACMatrixEntry> entries,
  /** 本次返回了几行。 */
  3: optional i32 count,
  /** 过滤后命中了几行。 */
  4: optional i32 total,
  /** 返回数是否少于命中数。 */
  5: optional bool truncated,
  /** 截断或没匹配上时的一句话说明。 */
  6: optional string note,
}
