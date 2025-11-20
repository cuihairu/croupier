#!/bin/bash

# Script to generate missing logic files
# This creates basic logic file skeletons that need to be filled with actual implementation

cat > /tmp/missing_logics.txt << 'EOF'
NewApprovalApproveLogic
NewApprovalGetLogic
NewApprovalRejectLogic
NewApprovalsListLogic
NewAuditLogic
NewCertificateAddLogic
NewCertificateAlertAddLogic
NewCertificateAlertsListLogic
NewCertificateCheckAllLogic
NewCertificateCheckLogic
NewCertificateDeleteLogic
NewCertificateDomainInfoLogic
NewCertificateExpiringLogic
NewCertificatesListLogic
NewCertificateStatsLogic
NewInvokeLogic
NewJobCancelLogic
NewJobResultLogic
NewJobStartLogic
NewMessageReadLogic
NewMessageSendLogic
NewMessagesListLogic
NewMessagesUnreadCountLogic
NewRoleCreateLogic
NewRoleDeleteLogic
NewRolePermissionsUpdateLogic
NewRolesListLogic
NewRoleUpdateLogic
NewRootLogic
NewSignedUrlLogic
NewStreamJobLogic
NewStreamMessagesLogic
NewUserCreateLogic
NewUserDeleteLogic
NewUserGameEnvsLogic
NewUserGameEnvsUpdateLogic
NewUserGamesLogic
NewUserGamesUpdateLogic
NewUserPasswordResetLogic
NewUsersListLogic
NewUserUpdateLogic
EOF

while read -r new_func; do
    # Remove "New" prefix and "Logic" suffix to get the struct name
    struct_name="${new_func#New}"

    # Convert to lowercase with underscores for filename
    filename=$(echo "$struct_name" | sed 's/\([A-Z]\)/_\1/g' | sed 's/^_//' | tr '[:upper:]' '[:lower:]')
    filename="internal/logic/${filename}.go"

    # Skip if file already exists
    if [ -f "$filename" ]; then
        echo "Skipping $filename (already exists)"
        continue
    fi

    # Generate method name (same as struct name without Logic suffix)
    method_name="${struct_name%Logic}"

    echo "Generating $filename for $struct_name..."

    cat > "$filename" << GEOF
// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type $struct_name struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func $new_func(ctx context.Context, svcCtx *svc.ServiceContext) *$struct_name {
	return &$struct_name{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TODO: Implement this method with actual business logic
func (l *$struct_name) $method_name() (resp interface{}, err error) {
	// TODO: Add your logic here and delete this line
	return nil, nil
}
GEOF

done < /tmp/missing_logics.txt

echo "Done! Generated logic files. Remember to implement the actual business logic."
