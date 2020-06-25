package pod

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/stepro/kdo/pkg/kubectl"
	"github.com/stepro/kdo/pkg/output"
)

func track(k kubectl.CLI, pod string, op output.Operation) func() {
	timestamp := time.Now().In(time.UTC).Format(time.RFC3339)

	return k.StartLines([]string{"get", "--raw=/api/v1/events?fieldSelector=involvedObject.name=" + pod + "&watch=1"}, func(line string) {
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return
		}
		obj := event["object"].(map[string]interface{})
		firstTimestamp, _ := obj["firstTimestamp"].(string)
		if firstTimestamp < timestamp {
			return
		}
		msg := obj["message"].(string)
		msg = strings.ToLower(msg[:1]) + msg[1:]
		if msg == "started container kdo-await-image-build" {
			msg = "awaiting image build"
		}
		op.Progress("%s", msg)
	}, nil)
}
