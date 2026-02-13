-- 节点表：存储函数、结构体、接口、变量、常量
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL,           -- 'func', 'struct', 'interface', 'package', 'var', 'const'
    name TEXT NOT NULL,           -- 完整限定名 (pkg.Name)
    package TEXT NOT NULL,        -- 包路径
    file TEXT NOT NULL,           -- 源文件路径
    line INTEGER NOT NULL,        -- 起始行号
    signature TEXT,               -- 函数签名
    doc TEXT                      -- 文档注释
);

-- 边表：存储调用关系
CREATE TABLE IF NOT EXISTS edges (
    id INTEGER PRIMARY KEY,
    from_id INTEGER NOT NULL,
    to_id INTEGER NOT NULL,
    kind TEXT NOT NULL,           -- 'calls', 'implements', 'references'
    call_site_file TEXT,          -- 调用发生的文件
    call_site_line INTEGER,       -- 调用发生的行号
    FOREIGN KEY (from_id) REFERENCES nodes(id),
    FOREIGN KEY (to_id) REFERENCES nodes(id)
);

CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
CREATE INDEX IF NOT EXISTS idx_nodes_package ON nodes(package);

