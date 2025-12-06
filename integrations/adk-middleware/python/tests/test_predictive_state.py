"""Tests for predictive state updates functionality."""

import pytest
from unittest.mock import MagicMock
import uuid

from ag_ui.core import EventType, CustomEvent
from ag_ui_adk.event_translator import EventTranslator
from ag_ui_adk.config import PredictStateMapping, normalize_predict_state


class TestPredictStateMapping:
    """Tests for PredictStateMapping configuration."""

    def test_predict_state_mapping_creation(self):
        """Test creating a PredictStateMapping instance."""
        mapping = PredictStateMapping(
            state_key="document",
            tool="write_document",
            tool_argument="content",
        )
        assert mapping.state_key == "document"
        assert mapping.tool == "write_document"
        assert mapping.tool_argument == "content"

    def test_predict_state_mapping_to_payload(self):
        """Test converting PredictStateMapping to payload format."""
        mapping = PredictStateMapping(
            state_key="document",
            tool="write_document",
            tool_argument="content",
        )
        payload = mapping.to_payload()
        assert payload == {
            "state_key": "document",
            "tool": "write_document",
            "tool_argument": "content",
        }


class TestNormalizePredictState:
    """Tests for normalize_predict_state helper."""

    def test_normalize_none(self):
        """Test normalizing None returns empty list."""
        result = normalize_predict_state(None)
        assert result == []

    def test_normalize_single_mapping(self):
        """Test normalizing a single mapping returns list."""
        mapping = PredictStateMapping(
            state_key="doc",
            tool="write",
            tool_argument="content",
        )
        result = normalize_predict_state(mapping)
        assert len(result) == 1
        assert result[0] == mapping

    def test_normalize_list_of_mappings(self):
        """Test normalizing a list of mappings."""
        mappings = [
            PredictStateMapping(state_key="doc1", tool="tool1", tool_argument="arg1"),
            PredictStateMapping(state_key="doc2", tool="tool2", tool_argument="arg2"),
        ]
        result = normalize_predict_state(mappings)
        assert len(result) == 2
        assert result == mappings


class TestEventTranslatorPredictState:
    """Tests for EventTranslator predictive state functionality."""

    @pytest.fixture
    def translator_with_predict_state(self):
        """Create translator with predictive state config."""
        return EventTranslator(
            predict_state=[
                PredictStateMapping(
                    state_key="document",
                    tool="write_document",
                    tool_argument="document",
                )
            ]
        )

    @pytest.fixture
    def translator_without_predict_state(self):
        """Create translator without predictive state config."""
        return EventTranslator()

    @pytest.mark.asyncio
    async def test_predict_state_event_emitted_for_matching_tool(
        self, translator_with_predict_state
    ):
        """Test that PredictState CustomEvent is emitted for matching tool."""
        # Create mock function call
        func_call = MagicMock()
        func_call.name = "write_document"
        func_call.id = str(uuid.uuid4())
        func_call.args = {"document": "Hello world"}

        events = []
        async for event in translator_with_predict_state._translate_function_calls(
            [func_call]
        ):
            events.append(event)

        # Should have: PredictState, ToolCallStart, ToolCallArgs, ToolCallEnd
        assert len(events) == 4

        # First event should be PredictState CustomEvent
        predict_state_event = events[0]
        assert isinstance(predict_state_event, CustomEvent)
        assert predict_state_event.type == EventType.CUSTOM
        assert predict_state_event.name == "PredictState"
        assert predict_state_event.value == [
            {
                "state_key": "document",
                "tool": "write_document",
                "tool_argument": "document",
            }
        ]

    @pytest.mark.asyncio
    async def test_no_predict_state_event_for_non_matching_tool(
        self, translator_with_predict_state
    ):
        """Test that no PredictState event is emitted for non-matching tool."""
        # Create mock function call for a different tool
        func_call = MagicMock()
        func_call.name = "other_tool"
        func_call.id = str(uuid.uuid4())
        func_call.args = {"data": "some data"}

        events = []
        async for event in translator_with_predict_state._translate_function_calls(
            [func_call]
        ):
            events.append(event)

        # Should only have: ToolCallStart, ToolCallArgs, ToolCallEnd
        assert len(events) == 3

        # None should be PredictState
        for event in events:
            if isinstance(event, CustomEvent):
                assert event.name != "PredictState"

    @pytest.mark.asyncio
    async def test_no_predict_state_event_without_config(
        self, translator_without_predict_state
    ):
        """Test that no PredictState event is emitted without config."""
        # Create mock function call
        func_call = MagicMock()
        func_call.name = "write_document"
        func_call.id = str(uuid.uuid4())
        func_call.args = {"document": "Hello world"}

        events = []
        async for event in translator_without_predict_state._translate_function_calls(
            [func_call]
        ):
            events.append(event)

        # Should only have: ToolCallStart, ToolCallArgs, ToolCallEnd
        assert len(events) == 3

        # None should be PredictState
        for event in events:
            if isinstance(event, CustomEvent):
                assert event.name != "PredictState"

    @pytest.mark.asyncio
    async def test_predict_state_event_only_emitted_once(
        self, translator_with_predict_state
    ):
        """Test that PredictState event is only emitted once per tool."""
        # Create two calls to the same tool
        func_call1 = MagicMock()
        func_call1.name = "write_document"
        func_call1.id = str(uuid.uuid4())
        func_call1.args = {"document": "First document"}

        func_call2 = MagicMock()
        func_call2.name = "write_document"
        func_call2.id = str(uuid.uuid4())
        func_call2.args = {"document": "Second document"}

        # First call
        events1 = []
        async for event in translator_with_predict_state._translate_function_calls(
            [func_call1]
        ):
            events1.append(event)

        # Second call
        events2 = []
        async for event in translator_with_predict_state._translate_function_calls(
            [func_call2]
        ):
            events2.append(event)

        # First call should have PredictState
        predict_state_count = sum(
            1
            for e in events1
            if isinstance(e, CustomEvent) and e.name == "PredictState"
        )
        assert predict_state_count == 1

        # Second call should NOT have PredictState
        predict_state_count = sum(
            1
            for e in events2
            if isinstance(e, CustomEvent) and e.name == "PredictState"
        )
        assert predict_state_count == 0

    @pytest.mark.asyncio
    async def test_predict_state_tracking_reset(self, translator_with_predict_state):
        """Test that reset clears predict state tracking."""
        # First call emits PredictState
        func_call = MagicMock()
        func_call.name = "write_document"
        func_call.id = str(uuid.uuid4())
        func_call.args = {"document": "First"}

        events1 = []
        async for event in translator_with_predict_state._translate_function_calls(
            [func_call]
        ):
            events1.append(event)

        # Reset translator
        translator_with_predict_state.reset()

        # Second call should emit PredictState again after reset
        func_call2 = MagicMock()
        func_call2.name = "write_document"
        func_call2.id = str(uuid.uuid4())
        func_call2.args = {"document": "Second"}

        events2 = []
        async for event in translator_with_predict_state._translate_function_calls(
            [func_call2]
        ):
            events2.append(event)

        # Both should have PredictState
        predict_state_count_1 = sum(
            1
            for e in events1
            if isinstance(e, CustomEvent) and e.name == "PredictState"
        )
        predict_state_count_2 = sum(
            1
            for e in events2
            if isinstance(e, CustomEvent) and e.name == "PredictState"
        )
        assert predict_state_count_1 == 1
        assert predict_state_count_2 == 1

    def test_multiple_predict_state_mappings(self):
        """Test translator with multiple predict state mappings."""
        translator = EventTranslator(
            predict_state=[
                PredictStateMapping(
                    state_key="document",
                    tool="write_document",
                    tool_argument="document",
                ),
                PredictStateMapping(
                    state_key="title",
                    tool="write_document",
                    tool_argument="title",
                ),
                PredictStateMapping(
                    state_key="other_state",
                    tool="other_tool",
                    tool_argument="data",
                ),
            ]
        )

        # Should have two tools in the mapping
        assert len(translator._predict_state_by_tool) == 2
        assert "write_document" in translator._predict_state_by_tool
        assert "other_tool" in translator._predict_state_by_tool

        # write_document should have two mappings
        assert len(translator._predict_state_by_tool["write_document"]) == 2

        # other_tool should have one mapping
        assert len(translator._predict_state_by_tool["other_tool"]) == 1
