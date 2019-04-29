package actions

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/actions/allocate"
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/framework"
)

func init() {
	framework.RegisterAction(allocate.New())
}
