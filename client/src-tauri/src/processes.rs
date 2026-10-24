use std::collections::HashMap;

use sysinfo::{ProcessRefreshKind, ProcessesToUpdate, System, UpdateKind};

use crate::api::{Game, ProcessEntry, RunningMatch};

fn refresh_system() -> System {
    let mut system = System::new();
    system.refresh_processes_specifics(
        ProcessesToUpdate::All,
        true,
        ProcessRefreshKind::nothing().with_exe(UpdateKind::Always),
    );
    system
}

/// All process names with occurrence counts, most common first.
pub fn list_processes() -> Vec<ProcessEntry> {
    let system = refresh_system();
    let mut counts: HashMap<String, u32> = HashMap::new();
    for process in system.processes().values() {
        let name = process.name().to_string_lossy().into_owned();
        if !name.is_empty() {
            *counts.entry(name).or_insert(0) += 1;
        }
    }
    let mut entries: Vec<ProcessEntry> = counts
        .into_iter()
        .map(|(name, count)| ProcessEntry { name, count })
        .collect();
    entries.sort_by(|a, b| b.count.cmp(&a.count).then_with(|| a.name.cmp(&b.name)));
    entries
}

fn exe_stem_of(process: &sysinfo::Process) -> String {
    process
        .exe()
        .and_then(|path| path.file_name())
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_default()
}

/// For every running process, check whether it matches any configured game.
pub fn running_matches(games: &[Game]) -> Vec<RunningMatch> {
    if games.is_empty() {
        return Vec::new();
    }
    let system = refresh_system();
    let mut matches = Vec::new();

    for (pid, process) in system.processes() {
        let name = process.name().to_string_lossy().into_owned();
        let exe_stem = exe_stem_of(process);
        let lower_name = name.to_lowercase();
        let lower_exe = exe_stem.to_lowercase();

        for game in games {
            let pattern = game.exe_pattern.trim().to_lowercase();
            if pattern.is_empty() {
                continue;
            }
            if lower_name.contains(&pattern) || lower_exe.contains(&pattern) {
                matches.push(RunningMatch {
                    pid: pid.as_u32(),
                    name,
                    exe_stem,
                    game_id: Some(game.id.clone()),
                    game_name: Some(game.name.clone()),
                });
                break; // one match per process is enough
            }
        }
    }
    matches
}
