#!/bin/env python3

import csv
import io
import random
import re
import subprocess


def get_old_chain_data():
    result = subprocess.run(
        ["/opt/3fs/admin_cli.sh", "list-chains"], capture_output=True, text=True
    )
    return result.stdout


def parse_old_chains(output):
    chain_map = {}
    for line in output.strip().splitlines()[1:]:
        parts = re.split(r"\s+", line.strip())
        if len(parts) >= 7:
            chain_id = parts[0]
            targets = [re.sub(r"\(.*?\)", "", t) for t in parts[5:]]
            chain_map[chain_id] = targets
    return chain_map


def get_new_chains():
    result = subprocess.run(
        ["docker", "exec", "-i", "3fs-mgmtd", "cat", "/output/generated_chains.csv"],
        capture_output=True,
        text=True,
    )
    return result.stdout


def parse_new_chains(csv_text):
    new_map = {}
    reader = csv.reader(io.StringIO(csv_text))
    next(reader)  # ignore the first line
    for row in reader:
        chain_id = row[0]
        targets = row[1:]
        new_map[chain_id] = targets
    return new_map


def get_new_targets():
    result = subprocess.run(
        ["docker", "exec", "-i", "3fs-mgmtd", "cat", "/output/create_target_cmd.txt"],
        capture_output=True,
        text=True,
    )
    return result.stdout


def parse_new_targets(output):
    # create-target --node-id 10001 --disk-index 0 --target-id 101000100101 --chain-id 900100001  --use-new-chunk-engine
    target_map = {}
    for line in output.strip().splitlines():
        if not line.startswith("create-target"):
            continue
        node_id = re.search(r"--node-id\s+(\d+)", line)
        disk_index = re.search(r"--disk-index\s+(\d+)", line)
        target_id = re.search(r"--target-id\s+(\d+)", line)
        chain_id = re.search(r"--chain-id\s+(\d+)", line)
        if not all([node_id, disk_index, target_id, chain_id]):
            print(f"# ⚠️ Warning: Unable to parse command: {line}")
            continue
        target_id = target_id.group(1)
        node_id = node_id.group(1)
        disk_index = disk_index.group(1)
        chain_id = chain_id.group(1)
        target_map[target_id] = (node_id, disk_index, chain_id)
    return target_map


def node_id_from_target(target_id):
    return target_id[:5]


def get_target_node_mapping():
    result = subprocess.run(
        ["/opt/3fs/admin_cli.sh", "list-targets"], capture_output=True, text=True
    )
    mapping = {}
    for line in result.stdout.strip().splitlines():
        if not line.startswith("101"):
            continue
        parts = re.split(r"\s+", line.strip())
        if len(parts) >= 7:
            target_id = parts[0]
            node_id = parts[5]
            disk_index = parts[6]
            mapping[target_id] = (node_id, disk_index)
    return mapping


def generate_commands(old_map, new_map, new_targets_map):
    now_target_node_map = get_target_node_mapping()
    commands = []

    for chain_id, new_targets in new_map.items():
        if chain_id not in old_map:
            continue
        old_targets = old_map[chain_id]
        if old_targets == new_targets:
            continue

        to_remove = list(set(old_targets) - set(new_targets))
        to_add = list(set(new_targets) - set(old_targets))
        to_add_with_node_id = {}
        for target in to_add:
            node_id, disk_index, chain_id = new_targets_map.get(target)
            if not node_id:
                print(
                    f"# ⚠️ Warning: Unable to find target {target} information to add to chain {chain_id}"
                )
                continue
            to_add_with_node_id[node_id] = target

        cmds = [f"# Chain {chain_id} migration"]

        for target in to_remove:
            if target not in now_target_node_map:
                print(
                    f"# ⚠️ Warning: Unable to find target {target} information to remove from chain {chain_id}"
                )
                continue
            node_id, disk_index = now_target_node_map.get(target)
            cmds += [
                f"/opt/3fs/admin_cli.sh offline-target --target-id {target}",
                f"/opt/3fs/admin_cli.sh update-chain --mode remove {chain_id} {target}",
                f"/opt/3fs/admin_cli.sh remove-target --node-id {node_id} --target-id {target}",
            ]

            if not to_add:
                continue

            add_target = random.choice(list(to_add))
            if (
                node_id in to_add_with_node_id
                and to_add_with_node_id[node_id] in to_add
            ):
                add_target = to_add_with_node_id[node_id]
            cmds += [
                f"/opt/3fs/admin_cli.sh create-target --node-id {node_id} --disk-index {disk_index} --target-id {add_target} --chain-id {chain_id}  --use-new-chunk-engine",
                f"/opt/3fs/admin_cli.sh update-chain --mode add {chain_id} {add_target}",
            ]
            to_add.remove(add_target)

        for target in to_add:
            if target not in new_targets_map:
                print(
                    f"# ⚠️ Warning: Unable to find target {target} information to add to chain {chain_id}"
                )
                continue
            node_id, disk_index, chain_id = new_targets_map.get(target)
            cmds += [
                f"/opt/3fs/admin_cli.sh create-target --node-id {node_id} --disk-index {disk_index} --target-id {target} --chain-id {chain_id}  --use-new-chunk-engine",
                f"/opt/3fs/admin_cli.sh update-chain --mode add {chain_id} {target}",
            ]

        commands.append("\n".join(cmds))
    return "\n\n".join(commands)


def main():
    old_chains_output = get_old_chain_data()
    old_map = parse_old_chains(old_chains_output)

    new_chains_csv = get_new_chains()
    new_map = parse_new_chains(new_chains_csv)

    new_targets_output = get_new_targets()
    new_targets_map = parse_new_targets(new_targets_output)

    result = generate_commands(old_map, new_map, new_targets_map)
    print(result)


if __name__ == "__main__":
    main()
