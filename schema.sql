create table if not exists Article(
	 Id integer primary key
	,Name text unique not null
	,Title text not null
	,RawTitle text not null
	,Content text not null
	,CreatedAt datetime not null
	,UpdatedAt datetime not null
);

