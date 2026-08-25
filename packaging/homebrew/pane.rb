class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.10"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.10/pane-v0.1.10-darwin-arm64.tar.gz"
      sha256 "cf626d4d4b74dbe17679ccd3161351359f3efc94e85e364d9ce4a655c4d6ffe2"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.10/pane-v0.1.10-darwin-amd64.tar.gz"
      sha256 "b287cd2e5fe4c6b52a4f068ce0e879957f8d102a3b9de99be20b1f74977e4297"
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
