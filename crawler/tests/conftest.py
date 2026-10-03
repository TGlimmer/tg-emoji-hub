"""pytest fixture：让 tests/ 能 import lib。"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
