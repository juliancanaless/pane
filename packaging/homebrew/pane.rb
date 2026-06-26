class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.4"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.4/pane-v0.1.4-darwin-arm64.tar.gz"
      sha256 "f3219220b885fcbc9f194708db7527e4ac5411befc9a6859bf533770e71dbf84"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.4/pane-v0.1.4-darwin-amd64.tar.gz"
      sha256 "4b789d9f79417d9e46a8adbdf1dd1ca6e72b18722900406d69e08a062ffb5297"
    end
  end

  depends_on "go" => :build if build.head?
  depends_on "rust" => :build if build.head?

  def install
    if build.head?
      ENV.prepend_path "PATH", Formula["rust"].opt_bin
      system "make", "build"
      bin.install "bin/pane"
      bin.install "bin/pane-analyze"
    else
      bin.install "pane"
      bin.install "pane-analyze"
    end
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
