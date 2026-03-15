package externalfunc

import "testing"

func TestDiscoverProviderOperations(t *testing.T) {
	got := DiscoverProviderOperations([]Binding{
		{
			BindingType: "provider",
			BindingKey:  "onepanel",
			Spec: map[string]any{
				"provider":   "onepanel",
				"operations": []any{"list_apps", "install_app"},
			},
		},
		{
			BindingType: "function",
			BindingKey:  "external.onepanel.upgrade_app",
		},
		{
			BindingType: "openapi",
			BindingKey:  "quicksdk",
			Spec: map[string]any{
				"provider": "quicksdk",
			},
		},
	})
	if len(got) != 2 {
		t.Fatalf("unexpected provider count: %+v", got)
	}
	if len(got["onepanel"]) != 3 ||
		got["onepanel"][0] != "list_apps" ||
		got["onepanel"][1] != "install_app" ||
		got["onepanel"][2] != "upgrade_app" {
		t.Fatalf("unexpected onepanel operations: %+v", got["onepanel"])
	}
	if len(got["quicksdk"]) != 1 || got["quicksdk"][0] != "invoke" {
		t.Fatalf("unexpected quicksdk operations: %+v", got["quicksdk"])
	}
}
