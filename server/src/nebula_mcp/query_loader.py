"""Utility for loading and caching SQL query files from nested directories."""

from pathlib import Path


class QueryLoader:
    """Reads `.sql` files from a folder tree and caches their contents.

    Supports nested directories using slash notation: QUERIES["entities/create"]
    """

    def __init__(self, path: Path | str) -> None:
        """Initialize the loader with a base path for SQL files."""

        self.path = Path(path)
        self.cache: dict[str, str] = {}

    def __getitem__(self, name: str) -> str:
        """Return cached SQL text for a query name.

        Args:
            name: Query name, optionally with path (e.g., "entities/create").

        Returns:
            str: SQL query text.

        Raises:
            FileNotFoundError: If query file doesn't exist.
        """

        if name not in self.cache:
            file_path = self.path / f"{name}.sql"

            if not file_path.exists():
                raise FileNotFoundError(f"Query file not found: {file_path}")

            self.cache[name] = file_path.read_text(encoding="utf-8")

        return self.cache[name]
