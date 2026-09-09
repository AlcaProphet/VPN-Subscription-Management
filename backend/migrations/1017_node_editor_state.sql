-- 1017_node_editor_state.sql — Build17 节点编辑统一保存契约。
-- 在 nodes 同一行保存最小当前状态、扩展负载、格式版本与编辑修订号。
-- 不创建独立 node_edit_states 表，不保存非激活分支、恢复副本或折叠状态。

ALTER TABLE nodes ADD COLUMN edit_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN state_format_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE nodes ADD COLUMN current_state_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE nodes ADD COLUMN extensions_json TEXT NOT NULL DEFAULT '{}';
