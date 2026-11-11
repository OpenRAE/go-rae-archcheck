"""Validate repository configuration before a change is published."""

import os
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]


class UniqueKeysLoader(yaml.SafeLoader):
    """Reject duplicate mapping keys in repository YAML."""

    pass


def construct_mapping(loader: yaml.SafeLoader, node: yaml.nodes.MappingNode) -> dict[object, object]:
    """Build a YAML mapping while rejecting duplicate keys."""
    result = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node)
        if key in result:
            raise ValueError(f"duplicate YAML key: {key}")
        result[key] = loader.construct_object(value_node)
    return result


UniqueKeysLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, construct_mapping
)


def main() -> None:
    """Parse workflow YAML and check the Ground Control binding."""
    paths = [ROOT / ".ground-control.yaml", *sorted((ROOT / ".github" / "workflows").glob("*.yml"))]
    for path in paths:
        with path.open(encoding="utf-8") as stream:
            yaml.load(stream, Loader=UniqueKeysLoader)
    config = yaml.load((ROOT / ".ground-control.yaml").read_text(encoding="utf-8"), Loader=UniqueKeysLoader)
    owner, repository = config["github_repo"].split("/", maxsplit=1)
    assert config["project"] == repository
    assert os.environ.get("GITHUB_REPOSITORY", config["github_repo"]) == config["github_repo"]
    assert config["workflow"]["base_branch"] == "dev"
    # OpenRAE projects use the raesystem SonarCloud organization, not the GitHub owner.
    assert owner == "OpenRAE"
    assert config["sonarcloud"]["organization"] == "raesystem"
    assert config["sonarcloud"]["analysis_check"] == "SonarCloud"
    properties = {}
    for line in (ROOT / "sonar-project.properties").read_text(encoding="utf-8").splitlines():
        if line and not line.startswith("#"):
            key, value = line.split("=", maxsplit=1)
            properties[key] = value
    assert properties["sonar.projectKey"] == config["sonarcloud"]["project_key"]
    assert properties["sonar.organization"] == config["sonarcloud"]["organization"]
    assert (ROOT / config["rules"]["plan_rules"]).is_file()
    title_workflow = yaml.load(
        (ROOT / ".github/workflows/pr-title-lint.yml").read_text(encoding="utf-8"),
        Loader=UniqueKeysLoader,
    )
    title_job = title_workflow["jobs"]["lint-title"]
    assert title_job["env"]["TITLE_TYPES"].split() == config["workflow"]["pr_title"]["types"]
    assert title_job["steps"][0]["with"]["requireScope"] is config["workflow"]["pr_title"]["require_scope"]
    assert title_job["env"]["TITLE_SUBJECT_PATTERN"] == config["workflow"]["pr_title"]["subject_pattern"]
    print(f"Validated {len(paths)} YAML files and Ground Control paths")


if __name__ == "__main__":
    main()
