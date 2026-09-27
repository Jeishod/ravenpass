cask "ravenpass" do
  version "0.1.0"
  sha256 "1c8d03ff0f2337ce5e2b568670e42ead617d90be6ac9959349f15589fec7e120"

  url "https://github.com/dortanes/ravenpass/releases/download/v#{version}/Ravenpass-#{version}-macos-arm64.dmg"
  name "Ravenpass"
  desc "Password manager with local vaults, passkeys, and TOTP codes"
  homepage "https://ravenpass.org/"

  livecheck do
    url :url
    strategy :github_latest
  end

  depends_on arch: :arm64
  depends_on macos: :ventura

  app "Ravenpass.app"

  caveats <<~EOS
    Ravenpass is currently in beta. Keep backups of your vault.
    System AutoFill requires macOS 15 or later.
  EOS
end
