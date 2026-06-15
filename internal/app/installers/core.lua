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
		root = e.name
		break
	end
end

if not root then
	return { error = "sing-box root directory not found in archive" }
end

local src = root .. "/" .. binary_name
local dst = binary_name

log.info("installing core " .. src .. " -> " .. dst)

local copy_err = fs.copy(asset.fs, src, dst)
if copy_err then
	return { error = "copy failed: " .. tostring(copy_err) }
end

return {}
