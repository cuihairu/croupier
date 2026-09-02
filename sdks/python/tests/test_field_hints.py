"""F14 验收测试：set_field_hint / set_field_widget 便捷层。"""

import json

import pytest

from croupier import FunctionDescriptor, set_field_hint, set_field_widget


def base() -> FunctionDescriptor:
    return FunctionDescriptor(id="player.ban", version="1.0.0")


def test_empty_schema_creates_object_skeleton():
    descriptor = set_field_widget(base(), "id", "Select")
    schema = descriptor.input_schema
    assert schema["type"] == "object"
    assert schema["properties"]["id"]["x-widget"] == "Select"


def test_existing_attributes_preserved_and_override():
    descriptor = set_field_widget(
        FunctionDescriptor(
            id="player.ban",
            input_schema={
                "type": "object",
                "properties": {"id": {"type": "string", "title": "玩家 ID", "x-widget": "Input"}},
            },
        ),
        "id",
        "TreeSelect",
    )
    prop = descriptor.input_schema["properties"]["id"]
    assert prop["title"] == "玩家 ID"
    assert prop["x-widget"] == "TreeSelect"


def test_options_source_object():
    descriptor = set_field_hint(
        base(),
        "id",
        "x-options-source",
        {"functionId": "player.list", "labelPath": "/items/*/name", "valuePath": "/items/*/id"},
    )
    prop = descriptor.input_schema["properties"]["id"]
    assert prop["x-options-source"]["functionId"] == "player.list"


def test_string_schema_still_mergeable():
    descriptor = set_field_widget(
        FunctionDescriptor(
            id="player.ban",
            input_schema=json.dumps({"type": "object", "properties": {"id": {"type": "string"}}}),
        ),
        "id",
        "Select",
    )
    prop = descriptor.input_schema["properties"]["id"]
    assert prop["x-widget"] == "Select"


def test_invalid_hint_rejected():
    with pytest.raises(ValueError, match="x- extension key"):
        set_field_hint(base(), "a", "widget", "Input")


def test_empty_field_rejected():
    with pytest.raises(ValueError, match="field key is required"):
        set_field_hint(base(), "  ", "x-widget", "Input")
