cask "ravenpass" do
  version "0.1.1"
  sha256 "7ef4911096d36cadcbf073c52162ba320b5c7106786ab4aa0aaebc781a62b9c3"

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
