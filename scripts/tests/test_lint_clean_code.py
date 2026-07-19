import sys
from pathlib import Path
from unittest.mock import MagicMock, mock_open, patch


# Add scripts to path
PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.append(str(Path(__file__).resolve().parent.parent))

import lint_clean_code  # noqa: E402


def test_extract_structs() -> None:
    # 1. Normal struct
    code_normal = """
    pub struct OvertimeSegment {
        start: NaiveTime,
        end: NaiveTime,
        multiplier: Decimal,
    }
    """
    structs = lint_clean_code.extract_structs(code_normal)
    assert len(structs) == 1
    assert structs[0]["name"] == "OvertimeSegment"
    assert structs[0]["type"] == "normal"
    assert "start: NaiveTime" in structs[0]["body"]

    # 2. Tuple struct
    code_tuple = """
    pub struct TupleObj(pub String, i32);
    """
    structs = lint_clean_code.extract_structs(code_tuple)
    assert len(structs) == 1
    assert structs[0]["name"] == "TupleObj"
    assert structs[0]["type"] == "tuple"
    assert "pub String, i32" in structs[0]["body"]

    # 3. Unit struct (ignored)
    code_unit = """
    pub struct UnitObj;
    """
    structs = lint_clean_code.extract_structs(code_unit)
    assert len(structs) == 0

    # 4. Tuple struct with nested parens
    code_nested_tuple = "struct Foo(pub Bar<(i32, u32)>, Baz);"
    structs = lint_clean_code.extract_structs(code_nested_tuple)
    assert len(structs) == 1
    assert "pub Bar<(i32, u32)>" in structs[0]["body"]

    # 5. Normal struct with nested braces
    code_nested_normal = "struct Foo { x: Bar { y: i32 } }"
    structs = lint_clean_code.extract_structs(code_nested_normal)
    assert len(structs) == 1
    assert "x: Bar { y: i32 }" in structs[0]["body"]

    # 6. Struct with trailing semicolon before brace
    code_semi_brace = "struct Foo; \n struct Bar { x: i32 }"
    structs = lint_clean_code.extract_structs(code_semi_brace)
    assert len(structs) == 1
    assert structs[0]["name"] == "Bar"


def test_check_private_fields() -> None:
    # 1. Normal struct with private fields -> 0 errors
    structs_ok = [
        {
            "name": "OvertimeSegment",
            "body": "start: NaiveTime,\nend: NaiveTime,",
            "line_no": 1,
            "type": "normal",
        }
    ]
    assert lint_clean_code.check_private_fields("test.rs", structs_ok) == 0

    # 2. Normal struct with public fields -> 1 error
    structs_err = [
        {
            "name": "OvertimeSegment",
            "body": "pub start: NaiveTime,\nend: NaiveTime,",
            "line_no": 1,
            "type": "normal",
        }
    ]
    assert lint_clean_code.check_private_fields("test.rs", structs_err) == 1

    # 3. Tuple struct with private fields -> 0 errors
    structs_tuple_ok = [
        {"name": "TupleObj", "body": "String, i32", "line_no": 1, "type": "tuple"}
    ]
    assert lint_clean_code.check_private_fields("test.rs", structs_tuple_ok) == 0

    # 4. Tuple struct with public fields -> 1 error
    structs_tuple_err = [
        {"name": "TupleObj", "body": "pub String, i32", "line_no": 1, "type": "tuple"}
    ]
    assert lint_clean_code.check_private_fields("test.rs", structs_tuple_err) == 1


def test_check_pure_domain_separation() -> None:
    # 1. Clean content
    clean_code = """
    pub struct Segment {
        start: NaiveTime,
    }
    """
    assert lint_clean_code.check_pure_domain_separation("test.rs", clean_code) == 0

    # 2. Content with PgConnection
    dirty_pg = """
    use crate::diesel_pool::PgConnection;
    """
    assert lint_clean_code.check_pure_domain_separation("test.rs", dirty_pg) == 1

    # 3. Content with TenantAuthToken
    dirty_token = """
    let auth = TenantAuthToken::system_override();
    """
    assert lint_clean_code.check_pure_domain_separation("test.rs", dirty_token) == 1

    # 4. Content with diesel::
    dirty_diesel = """
    use diesel::prelude::*;
    """
    assert lint_clean_code.check_pure_domain_separation("test.rs", dirty_diesel) == 1

    # 5. Content with crate::repositories
    dirty_repo = """
    use crate::repositories::UserRepository;
    """
    assert lint_clean_code.check_pure_domain_separation("test.rs", dirty_repo) == 1


@patch("lint_clean_code.Path")
@patch("sys.exit")
def test_main_success(mock_exit: MagicMock, mock_path_cls: MagicMock) -> None:
    mock_domain_dir = MagicMock()
    mock_domain_dir.exists.return_value = True

    mock_file = MagicMock()
    mock_file.name = "policy.rs"
    mock_file.relative_to.return_value = Path("apiserver/src/domain/policy.rs")

    # mod.rs should be skipped by the linter
    mock_mod_file = MagicMock()
    mock_mod_file.name = "mod.rs"
    mock_mod_file.relative_to.return_value = Path("apiserver/src/domain/mod.rs")

    mock_domain_dir.glob.return_value = [mock_file, mock_mod_file]
    mock_path_cls.return_value = mock_domain_dir

    file_content = """
    pub struct Policy {
        id: Uuid,
    }
    """
    mock_opened_file = mock_open(read_data=file_content)
    mock_file.open = mock_opened_file

    lint_clean_code.main()
    mock_exit.assert_called_once_with(0)


@patch("lint_clean_code.Path")
@patch("sys.exit")
def test_main_failure(mock_exit: MagicMock, mock_path_cls: MagicMock) -> None:
    mock_domain_dir = MagicMock()
    mock_domain_dir.exists.return_value = True

    mock_file = MagicMock()
    mock_file.name = "policy.rs"
    mock_file.relative_to.return_value = Path("apiserver/src/domain/policy.rs")

    mock_domain_dir.glob.return_value = [mock_file]
    mock_path_cls.return_value = mock_domain_dir

    # Has a public field in normal struct
    file_content = """
    pub struct Policy {
        pub id: Uuid,
    }
    """
    mock_opened_file = mock_open(read_data=file_content)
    mock_file.open = mock_opened_file

    lint_clean_code.main()
    mock_exit.assert_called_once_with(1)


@patch("lint_clean_code.Path")
@patch("sys.exit")
def test_main_dir_missing(mock_exit: MagicMock, mock_path_cls: MagicMock) -> None:
    mock_domain_dir = MagicMock()
    mock_domain_dir.exists.return_value = False
    mock_path_cls.return_value = mock_domain_dir

    lint_clean_code.main()
    mock_exit.assert_called_once_with(1)


class TestMainFunction:
    def test_main_with_patches(self):
        import lint_clean_code
        from unittest.mock import MagicMock, patch

        mock_dir = MagicMock()
        mock_dir.exists.return_value = True
        mock_file = MagicMock()
        mock_file.name = "policy.rs"
        mock_file.relative_to.return_value = Path("apiserver/src/domain/policy.rs")
        mock_dir.glob.return_value = [mock_file]
        with patch.object(lint_clean_code, "Path", return_value=mock_dir):
            with patch("sys.exit") as mock_exit:
                mock_file.open = MagicMock()
                mock_file.open.return_value.__enter__.return_value.read.return_value = (
                    "pub struct Policy { id: Uuid }"
                )
                lint_clean_code.main()
                mock_exit.assert_called_once_with(0)
