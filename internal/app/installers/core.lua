-- Install script for the sing-box core release asset.
-- The release archive has a single root directory like
-- sing-box-<version>-<os>-<arch>/ which contains the binary.

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

local src = root .. "/" .. binary_name
local dst = binary_name
local old_dst = dst .. ".old"

log.info("installing core " .. src .. " -> " .. dst)

-- Rotate any existing binary out of the way first. This allows us to
-- replace a running executable without hitting "text file busy" (Unix)
-- or "file in use" (Windows) errors.
if fs.exists(dst) then
	if fs.exists(old_dst) then
		local remove_err = fs.remove(old_dst)
		if remove_err then
			return { error = "cannot remove stale old binary: " .. tostring(remove_err) }
		end
	end
	local rename_err = fs.rename(dst, old_dst)
	if rename_err then
		return { error = "cannot rotate existing binary: " .. tostring(rename_err) }
	end
end

local copy_err = fs.copy(asset.fs, src, dst)
if copy_err then
	-- Attempt to roll back to the previous binary on failure.
	if fs.exists(old_dst) and not fs.exists(dst) then
		_ = fs.rename(old_dst, dst)
	end
	return { error = "copy failed: " .. tostring(copy_err) }
end

-- Cleanup the rotated binary on success. Ignore errors here; the old
-- file will be removed on the next update.
if fs.exists(old_dst) then
	_ = fs.remove(old_dst)
end

-- Verify the binary was actually written.
if not fs.exists(dst) then
	return { error = "destination binary missing after copy: " .. dst }
end

return {}
