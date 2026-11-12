-- Install script for the sing-box core release asset.
-- The release archive has a single root directory like
-- sing-box-<version>-<os>-<arch>/ containing the binary and any companion
-- files (licenses, shared libraries, etc.). Everything from that root is
-- extracted into the "core" subdirectory of the data dir, so the core keeps
-- its own files together instead of a single flat binary.

local binary_name = "sing-box"
if platform.os == "windows" then
	binary_name = "sing-box.exe"
end

local entries, err = asset.fs.list_dir(".")
if err then
	return { error = "cannot list archive root: " .. tostring(err) }
end

local root = nil
for _, e in ipairs(entries) do
	if e.is_dir and e.name:match("^sing%-box%-") then
		root = e.name:gsub("/$", "")
		break
	end
end

if not root then
	return { error = "sing-box root directory not found in archive" }
end

local dst_dir = "core"
local old_dir = dst_dir .. ".old"

log.info("installing core " .. root .. " -> " .. dst_dir)

-- Rotate any existing core directory out of the way first. This lets us
-- replace a running executable without hitting "text file busy" (Unix) or
-- "file in use" (Windows) errors.
if fs.exists(dst_dir) then
	if fs.exists(old_dir) then
		local remove_err = fs.remove(old_dir)
		if remove_err then
			return { error = "cannot remove stale old core dir: " .. tostring(remove_err) }
		end
	end
	local rename_err = fs.rename(dst_dir, old_dir)
	if rename_err then
		return { error = "cannot rotate existing core dir: " .. tostring(rename_err) }
	end
end

local mkdir_err = fs.mkdir(dst_dir)
if mkdir_err then
	return { error = "cannot create core dir: " .. tostring(mkdir_err) }
end

-- Copy every file from the archive root into core/ (binary + companion
-- files such as LICENSE and any bundled libraries).
local files, list_err = asset.fs.list_dir(root)
if list_err then
	return { error = "cannot list core archive root: " .. tostring(list_err) }
end
for _, e in ipairs(files) do
	local name = e.name:gsub("/$", "")
	if not e.is_dir then
		local src = root .. "/" .. name
		local dst = dst_dir .. "/" .. name
		local copy_err = fs.copy(asset.fs, src, dst)
		if copy_err then
			-- Attempt to roll back to the previous core on failure.
			if fs.exists(old_dir) and not fs.exists(dst_dir) then
				_ = fs.rename(old_dir, dst_dir)
			end
			return { error = "copy failed: " .. tostring(copy_err) }
		end
	end
end

-- Cleanup the rotated directory on success. Ignore errors here; leftovers
-- are removed on the next update.
if fs.exists(old_dir) then
	_ = fs.remove(old_dir)
end

-- Verify the binary was actually written.
local dst_binary = dst_dir .. "/" .. binary_name
if not fs.exists(dst_binary) then
	return { error = "destination binary missing after copy: " .. dst_binary }
end

return {}
