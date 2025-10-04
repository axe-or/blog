create table if not exists Article(
	 Id integer primary key
	,Name text unique not null
	,Title text
	,Content text not null
	,CreatedAt datetime not null
	,UpdatedAt datetime not null
);

