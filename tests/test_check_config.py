"""The configuration check accepts the configuration of this repository."""

import io
import sys
import unittest
from unittest.mock import patch

from scripts.check_config import main


class CheckConfigTest(unittest.TestCase):
    """Run the configuration check on the repository files."""

    def test_repository_configuration_passes(self):
        with patch.object(sys, "stdout", io.StringIO()) as output:
            main()
        self.assertIn("Validated", output.getvalue())


if __name__ == "__main__":
    unittest.main()
