This program takes two flags - `src` and `dst` and copies files that are present in `src` but not in `dst` and files
that are a different size (I tried using hashes to determine whether a file is different, but it was painfully slow).
Also, folders that are named `dont_mirror` will be ignored.

There's also an optional `c` flag that turns on "cleaning mode". In this mode, every file and directory that is present
in `dst` but not in `src` will be deleted. (Files with different sizes will be left alone)

I use this program for my personal use, so it isn't the fastest thing ever written, but it can handle a few million
files just fine.